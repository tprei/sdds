package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeS3Client struct {
	putCalls, headCalls                int
	putErr, headErr, getErr, deleteErr error
	headOutput                         *s3.HeadObjectOutput
	getOutput                          *s3.GetObjectOutput
	headContextErr                     error
	headDeadline                       time.Time
	headChecksumMode                   s3types.ChecksumMode
}

func (fake *fakeS3Client) PutObject(_ context.Context, _ *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	fake.putCalls++
	return &s3.PutObjectOutput{}, fake.putErr
}
func (fake *fakeS3Client) HeadObject(ctx context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	fake.headCalls++
	fake.headContextErr = ctx.Err()
	fake.headDeadline, _ = ctx.Deadline()
	fake.headChecksumMode = input.ChecksumMode
	return fake.headOutput, fake.headErr
}
func (fake *fakeS3Client) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return fake.getOutput, fake.getErr
}
func (fake *fakeS3Client) DeleteObject(_ context.Context, _ *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, fake.deleteErr
}
func newFakeStore(_ *testing.T, fake *fakeS3Client) *S3Store {
	return &S3Store{client: fake, bucket: "bucket", timeout: time.Second}
}
func testKey() ObjectKey { return "note-images/018f3a7b-4e2c-7abc-8def-0123456789ab" }
func testInput(body []byte) PutObject {
	return PutObject{Key: testKey(), Body: bytes.NewReader(body), Size: int64(len(body)), SHA256: sha256.Sum256(body), ContentType: "image/jpeg"}
}
func matchingHead(input PutObject) *s3.HeadObjectOutput {
	digest := hex.EncodeToString(input.SHA256[:])
	return &s3.HeadObjectOutput{ContentLength: &input.Size, Metadata: map[string]string{digestMetadataKey: digest}}
}
func responseError(status int) error {
	return &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}}, Err: errors.New("response")}
}

type zeroThenExtra struct {
	bytes.Reader
	zero bool
}

func (reader *zeroThenExtra) Read(buffer []byte) (int, error) {
	if reader.Len() == 1 && !reader.zero {
		reader.zero = true
		return 0, nil
	}
	return reader.Reader.Read(buffer)
}
func TestPutValidatesBytesDigestAndRewinds(t *testing.T) {
	body, digest := []byte("image bytes"), sha256.Sum256([]byte("image bytes"))
	for _, key := range []ObjectKey{"", "note-images/not-a-uuid", "note-images/../01234567-89ab-cdef-0123-456789abcdef", "system/readiness", "/note-images/01234567-89ab-cdef-0123-456789abcdef"} {
		if err := ValidateObjectKey(key); !errors.Is(err, ErrInvalidObjectKey) {
			t.Errorf("ValidateObjectKey(%q) = %v", key, err)
		}
	}
	tests := []struct {
		name string
		body io.ReadSeeker
		size int64
		hash [32]byte
	}{
		{"short", bytes.NewReader(body), int64(len(body) + 1), digest},
		{"long", bytes.NewReader(body), int64(len(body) - 1), digest},
		{"extra after zero,nil", &zeroThenExtra{Reader: *bytes.NewReader(append(body, '!'))}, int64(len(body)), digest},
		{"digest", bytes.NewReader(body), int64(len(body)), [32]byte{1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeS3Client{}
			input := PutObject{Key: testKey(), Body: test.body, Size: test.size, SHA256: test.hash}
			if err := newFakeStore(t, fake).Put(context.Background(), input); !errors.Is(err, ErrObjectIntegrity) || fake.putCalls != 0 {
				t.Fatalf("Put error = %v, calls = %d", err, fake.putCalls)
			}
		})
	}
	fake := &fakeS3Client{}
	input := testInput(body)
	_, _ = input.Body.Seek(3, io.SeekStart)
	if err := newFakeStore(t, fake).Put(context.Background(), input); err != nil || fake.putCalls != 1 {
		t.Fatalf("valid Put error = %v, calls = %d", err, fake.putCalls)
	}
}
func TestShouldReconcilePut(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"dropped response", io.ErrUnexpectedEOF, true},
		{"connection reset", fmt.Errorf("put: %w", syscall.ECONNRESET), true},
		{"canceled", context.Canceled, true},
		{"deadline", context.DeadlineExceeded, true},
		{"transient API", &smithy.GenericAPIError{Code: "SlowDown"}, true},
		{"bad digest", &smithy.GenericAPIError{Code: "BadDigest"}, false},
	} {
		if got := shouldReconcilePut(test.err); got != test.want {
			t.Errorf("%s: shouldReconcilePut = %v, want %v", test.name, got, test.want)
		}
	}
}
func TestCanceledPutReconcilesWithFreshBoundedContext(t *testing.T) {
	input := testInput([]byte("same"))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, test := range []struct {
		name string
		ctx  context.Context
		err  error
	}{{"canceled", canceled, context.Canceled}, {"deadline", expired, context.DeadlineExceeded}} {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeS3Client{putErr: test.err, headOutput: matchingHead(input)}
			if err := newFakeStore(t, fake).Put(test.ctx, input); err != nil || fake.headCalls != 1 || fake.headContextErr != nil || fake.headDeadline.IsZero() {
				t.Fatalf("Put = %v, head calls = %d, context err = %v, deadline = %v", err, fake.headCalls, fake.headContextErr, fake.headDeadline)
			}
		})
	}
}
func TestReconciliationMismatchAndProviderMapping(t *testing.T) {
	input := testInput([]byte("same"))
	fake := &fakeS3Client{putErr: &smithy.GenericAPIError{Code: "PreconditionFailed"}, headOutput: matchingHead(input)}
	*fake.headOutput.ContentLength++
	if err := newFakeStore(t, fake).Put(context.Background(), input); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("mismatched reconciliation error = %v, want exists", err)
	}
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{"NoSuchKey", &smithy.GenericAPIError{Code: "NoSuchKey"}, ErrObjectNotFound},
		{"NotFound", &smithy.GenericAPIError{Code: "NotFound"}, ErrObjectUnavailable},
		{"NoSuchBucket", &smithy.GenericAPIError{Code: "NoSuchBucket"}, ErrObjectUnavailable},
		{"raw 404", responseError(404), ErrObjectUnavailable},
	} {
		if got := mapProviderError(test.err); !errors.Is(got, test.want) {
			t.Errorf("%s: mapProviderError = %v, want %v", test.name, got, test.want)
		}
	}
	fake = &fakeS3Client{getErr: &smithy.GenericAPIError{Code: "NoSuchKey"}}
	if _, err := newFakeStore(t, fake).Open(context.Background(), testKey()); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Open missing error = %v", err)
	}
	fake = &fakeS3Client{deleteErr: &smithy.GenericAPIError{Code: "NoSuchKey"}}
	if err := newFakeStore(t, fake).Delete(context.Background(), testKey()); err != nil {
		t.Fatalf("Delete missing error = %v", err)
	}
}
func TestOpenStreamsProviderBody(t *testing.T) {
	input := testInput([]byte("bytes"))
	fake := &fakeS3Client{getOutput: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader([]byte("bytes"))), ContentLength: &input.Size, Metadata: map[string]string{digestMetadataKey: hex.EncodeToString(input.SHA256[:])}}}
	object, err := newFakeStore(t, fake).Open(context.Background(), input.Key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(object.Body)
	if err != nil || string(got) != "bytes" {
		t.Fatalf("read object = %q, err = %v", got, err)
	}
}
func TestValidateEndpoint(t *testing.T) {
	for _, test := range []struct {
		raw string
		ok  bool
	}{{"http://rustfs:9000", true}, {"http://rustfs:9000/", true}, {"http://rustfs:9000/api", false}} {
		if got := validateEndpoint(test.raw) == nil; got != test.ok {
			t.Errorf("validateEndpoint(%q) = %v, want %v", test.raw, got, test.ok)
		}
	}
}

func validReadinessHead() *s3.HeadObjectOutput {
	return &s3.HeadObjectOutput{ContentLength: aws.Int64(readinessSentinelContentLength), ChecksumSHA256: aws.String(readinessSentinelChecksumSHA256), Metadata: map[string]string{digestMetadataKey: readinessSentinelDigest}}
}
func TestVerifyReadinessContract(t *testing.T) {
	tests := []struct {
		name string
		err  error
		edit func(*s3.HeadObjectOutput)
	}{
		{"valid", nil, nil},
		{"empty response", ErrObjectIntegrity, nil},
		{"missing length", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) { h.ContentLength = nil }},
		{"wrong length", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) { n := readinessSentinelContentLength + 1; h.ContentLength = &n }},
		{"missing checksum", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) { h.ChecksumSHA256 = nil }},
		{"same-length corrupt checksum", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) {
			payload := []byte(readinessSentinelPayload)
			payload[len(payload)-2] = '0'
			checksum := sha256.Sum256(payload)
			value := base64.StdEncoding.EncodeToString(checksum[:])
			h.ChecksumSHA256 = &value
		}},
		{"missing metadata", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) { h.Metadata = nil }},
		{"unexpected metadata", ErrObjectIntegrity, func(h *s3.HeadObjectOutput) { h.Metadata["extra"] = "value" }},
		{"provider missing", ErrObjectNotFound, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeS3Client{headOutput: validReadinessHead()}
			if test.name == "empty response" {
				fake.headOutput = nil
			} else if test.name == "provider missing" {
				fake.headErr = &smithy.GenericAPIError{Code: "NoSuchKey"}
			} else if test.edit != nil {
				test.edit(fake.headOutput)
			}
			err := newFakeStore(t, fake).VerifyReadiness(context.Background())
			if !errors.Is(err, test.err) {
				t.Fatalf("VerifyReadiness = %v, want %v", err, test.err)
			}
			if test.name == "valid" && (fake.headCalls != 1 || fake.headContextErr != nil || fake.headDeadline.IsZero() || fake.headChecksumMode != s3types.ChecksumModeEnabled) {
				t.Fatalf("head calls=%d context=%v deadline=%v mode=%q", fake.headCalls, fake.headContextErr, fake.headDeadline, fake.headChecksumMode)
			}
		})
	}
}
func TestNewS3StoreSDKWireContract(t *testing.T) {
	body, input := []byte("signed image"), testInput([]byte("signed image"))
	access, secret := "access-key", "secret-key"
	var puts, heads int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			puts++
			payload, _ := io.ReadAll(r.Body)
			if r.URL.Path != "/bucket/"+string(input.Key) || strings.HasPrefix(r.Host, "bucket.") || !bytes.Equal(payload, body) || r.ContentLength != int64(len(body)) {
				t.Errorf("PUT wire request path=%q host=%q body=%q length=%d", r.URL.Path, r.Host, payload, r.ContentLength)
			}
			if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential="+access+"/") || r.Header.Get("If-None-Match") != "*" || r.Header.Get("Content-Type") != input.ContentType || r.Header.Get("X-Amz-Meta-Sha256") != hex.EncodeToString(input.SHA256[:]) || r.Header.Get("X-Amz-Checksum-Sha256") != base64.StdEncoding.EncodeToString(input.SHA256[:]) {
				t.Errorf("PUT signed/immutable headers missing: %#v", r.Header)
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = io.WriteString(w, "<Error><Code>PreconditionFailed</Code></Error>")
		case http.MethodHead:
			heads++
			if r.URL.Path != "/bucket/"+string(input.Key) {
				t.Errorf("HEAD path = %q", r.URL.Path)
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			w.Header().Set("X-Amz-Meta-Sha256", hex.EncodeToString(input.SHA256[:]))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != "rustfs:9000" {
				return nil, fmt.Errorf("unexpected dial address %q", address)
			}
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	defer transport.CloseIdleConnections()
	dir := t.TempDir()
	accessFile, secretFile := filepath.Join(dir, "access"), filepath.Join(dir, "secret")
	if err := os.WriteFile(accessFile, []byte(access), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretFile, []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := newS3Store(context.Background(), Config{Endpoint: "http://rustfs:9000", Region: "us-east-1", Bucket: "bucket", UsePathStyle: true, AccessKeyFile: accessFile, SecretKeyFile: secretFile, Timeout: time.Second, RetryMaxAttempts: 1}, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), input); err != nil {
		t.Fatalf("Put = %v", err)
	}
	if puts != 1 || heads != 1 {
		t.Fatalf("wire calls = put %d head %d, want one each", puts, heads)
	}
}

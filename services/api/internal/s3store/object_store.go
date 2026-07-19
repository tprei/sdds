package s3store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/media"
)

const (
	objectKeyPrefix   = "note-images/"
	digestMetadataKey = "sha256"
)

func newStore(ctx context.Context, config Config, httpClient aws.HTTPClient) (*Store, error) {
	if !config.loaded {
		return nil, errors.New("S3 configuration is not loaded")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(config.region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.accessKey, config.secretKey, "")),
	}
	if httpClient != nil {
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(httpClient))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(config.endpoint)
		options.UsePathStyle = config.usePathStyle
		options.Retryer = retry.NewStandard(func(retryOptions *retry.StandardOptions) {
			retryOptions.MaxAttempts = config.retryMaxAttempts
		})
	})
	return &Store{client: client, bucket: config.bucket, timeout: config.timeout}, nil
}

func validateObjectKey(key media.ObjectKey) error {
	value := string(key)
	if len(value) != len(objectKeyPrefix)+36 || !strings.HasPrefix(value, objectKeyPrefix) || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
		return fmt.Errorf("%w: %w", media.ErrInvalidObjectKey, media.ErrObjectIntegrity)
	}
	id := value[len(objectKeyPrefix):]
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return fmt.Errorf("%w: %w", media.ErrInvalidObjectKey, media.ErrObjectIntegrity)
	}
	return nil
}
func (store *Store) Put(ctx context.Context, input media.PutObject) error {
	if err := validateObjectKey(input.Key); err != nil {
		return err
	}
	if err := validateAndRewind(input); err != nil {
		return err
	}
	digest := hex.EncodeToString(input.SHA256[:])
	request := &s3.PutObjectInput{
		Bucket:         aws.String(store.bucket),
		Key:            aws.String(string(input.Key)),
		Body:           input.Body,
		ContentLength:  aws.Int64(input.Size),
		IfNoneMatch:    aws.String("*"),
		ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(input.SHA256[:])),
		Metadata:       map[string]string{digestMetadataKey: digest},
	}
	if input.ContentType != "" {
		request.ContentType = aws.String(input.ContentType)
	}
	requestCtx, cancel := store.withTimeout(ctx)
	_, err := store.client.PutObject(requestCtx, request)
	cancel()
	if err == nil {
		return nil
	}
	if !shouldReconcilePut(err) {
		return mapProviderError(err)
	}
	return store.reconcilePut(input.Key, input.Size, input.SHA256)
}
func validateAndRewind(input media.PutObject) error {
	if input.Body == nil || input.Size < 0 {
		return media.ErrObjectIntegrity
	}
	if _, err := input.Body.Seek(0, io.SeekStart); err != nil {
		return media.ErrObjectIntegrity
	}
	hasher := sha256.New()
	count, err := io.CopyN(hasher, input.Body, input.Size)
	if err != nil || count != input.Size {
		return rewindIntegrity(input.Body)
	}
	var extra [1]byte
	n, err := io.ReadFull(input.Body, extra[:])
	if n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return rewindIntegrity(input.Body)
	}
	if !bytes.Equal(hasher.Sum(nil), input.SHA256[:]) {
		return rewindIntegrity(input.Body)
	}
	if _, err := input.Body.Seek(0, io.SeekStart); err != nil {
		return media.ErrObjectIntegrity
	}
	return nil
}
func rewindIntegrity(body io.Seeker) error {
	_, _ = body.Seek(0, io.SeekStart)
	return media.ErrObjectIntegrity
}
func (store *Store) reconcilePut(key media.ObjectKey, size int64, digest [32]byte) error {
	reconcileCtx, cancel := context.WithTimeout(context.Background(), store.timeout)
	defer cancel()
	output, err := store.client.HeadObject(reconcileCtx, &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	if err != nil {
		mapped := mapProviderError(err)
		if errors.Is(mapped, media.ErrObjectNotFound) {
			return media.ErrObjectUnavailable
		}
		return mapped
	}
	if output == nil || output.ContentLength == nil {
		return media.ErrObjectIntegrity
	}
	metadataDigest, ok := metadataDigest(output.Metadata)
	if !ok || *output.ContentLength != size || metadataDigest != hex.EncodeToString(digest[:]) {
		return media.ErrObjectExists
	}
	return nil
}
func metadataDigest(metadata map[string]string) (string, bool) {
	value, ok := metadata[digestMetadataKey]
	if !ok || value != strings.ToLower(value) || len(value) != sha256.Size*2 {
		return "", false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", false
	}
	return value, true
}
func (store *Store) Open(ctx context.Context, key media.ObjectKey) (media.Object, error) {
	if err := validateObjectKey(key); err != nil {
		return media.Object{}, err
	}
	requestCtx, cancel := store.withTimeout(ctx)
	output, err := store.client.GetObject(requestCtx, &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	if err != nil {
		cancel()
		return media.Object{}, mapProviderError(err)
	}
	if output == nil || output.Body == nil || output.ContentLength == nil {
		closeGetObject(output, cancel)
		return media.Object{}, media.ErrObjectIntegrity
	}
	digestText, ok := metadataDigest(output.Metadata)
	if !ok {
		closeGetObject(output, cancel)
		return media.Object{}, media.ErrObjectIntegrity
	}
	digestBytes, err := hex.DecodeString(digestText)
	if err != nil {
		closeGetObject(output, cancel)
		return media.Object{}, media.ErrObjectIntegrity
	}
	if output.ChecksumSHA256 != nil {
		checksum, checksumErr := base64.StdEncoding.DecodeString(*output.ChecksumSHA256)
		if checksumErr != nil || !bytes.Equal(checksum, digestBytes) {
			closeGetObject(output, cancel)
			return media.Object{}, media.ErrObjectIntegrity
		}
	}
	var digest [32]byte
	copy(digest[:], digestBytes)
	return media.Object{Body: &cancelReadCloser{body: output.Body, cancel: cancel}, Size: *output.ContentLength, SHA256: digest}, nil
}
func closeGetObject(output *s3.GetObjectOutput, cancel context.CancelFunc) {
	if output != nil && output.Body != nil {
		_ = output.Body.Close()
	}
	cancel()
}

type cancelReadCloser struct {
	body      io.ReadCloser
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
}

func (reader *cancelReadCloser) Read(buffer []byte) (int, error) { return reader.body.Read(buffer) }
func (reader *cancelReadCloser) Close() error {
	reader.closeOnce.Do(func() {
		reader.closeErr = reader.body.Close()
		reader.cancel()
	})
	return reader.closeErr
}
func (store *Store) Delete(ctx context.Context, key media.ObjectKey) error {
	if err := validateObjectKey(key); err != nil {
		return err
	}
	requestCtx, cancel := store.withTimeout(ctx)
	_, err := store.client.DeleteObject(requestCtx, &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	cancel()
	if err == nil {
		return nil
	}
	mapped := mapProviderError(err)
	if errors.Is(mapped, media.ErrObjectNotFound) {
		return nil
	}
	return mapped
}
func shouldReconcilePut(err error) bool {
	if mapProviderError(err) == media.ErrObjectExists {
		return true
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch strings.ToLower(apiError.ErrorCode()) {
		case "internalerror", "serviceunavailable", "slowdown", "requesttimeout", "requesttimeoutexception", "requestcanceled", "requestcanceledexception", "throttling", "throttlingexception", "toomanyrequests":
			return true
		default:
			return false
		}
	}
	var responseError *smithyhttp.ResponseError
	if errors.As(err, &responseError) {
		return responseError.HTTPStatusCode() >= 500
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
func mapProviderError(err error) error {
	if err == nil {
		return nil
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch strings.ToLower(apiError.ErrorCode()) {
		case "nosuchkey":
			return media.ErrObjectNotFound
		case "baddigest", "invaliddigest", "checksummismatch", "invalidchecksum":
			return media.ErrObjectIntegrity
		case "preconditionfailed", "conditionalrequestconflict":
			return media.ErrObjectExists
		default:
			return media.ErrObjectUnavailable
		}
	}
	var responseError *smithyhttp.ResponseError
	if errors.As(err, &responseError) {
		switch responseError.HTTPStatusCode() {
		case 400:
			return media.ErrObjectIntegrity
		case 409, 412:
			return media.ErrObjectExists
		default:
			return media.ErrObjectUnavailable
		}
	}
	return media.ErrObjectUnavailable
}

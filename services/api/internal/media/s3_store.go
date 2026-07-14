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
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	objectKeyPrefix       = "note-images/"
	digestMetadataKey     = "sha256"
	defaultRequestTimeout = 15 * time.Second
	defaultMaxAttempts    = 3
	maxRequestTimeout     = 5 * time.Minute
	maxRetryAttempts      = 10
)

const (
	readinessSentinelKey            ObjectKey = "system/readiness"
	readinessSentinelPayload                  = "sdds-media-ready-v1\n"
	readinessSentinelContentLength  int64     = int64(len(readinessSentinelPayload))
	readinessSentinelDigest                   = "5aff33ce5e386989939a8a504923897432db5b5a818518ccd876dadf2ad7398f"
	readinessSentinelChecksumSHA256           = "Wv8zzl44aYmTmopQSSOJdDLbW1qBhRjM2Hba3yrXOY8="
)

type Config struct {
	Endpoint, Region, Bucket     string
	UsePathStyle                 bool
	AccessKeyFile, SecretKeyFile string
	Timeout                      time.Duration
	RetryMaxAttempts             int
}
type s3Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}
type S3Store struct {
	client  s3Client
	bucket  string
	timeout time.Duration
}

func NewS3Store(ctx context.Context, cfg Config) (*S3Store, error) {
	return newS3Store(ctx, cfg, nil)
}

func newS3Store(ctx context.Context, cfg Config, httpClient aws.HTTPClient) (*S3Store, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := validateEndpoint(cfg.Endpoint); err != nil {
		return nil, err
	}
	accessKey, err := readCredentialFile(cfg.AccessKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read access key file: %w", err)
	}
	secretKey, err := readCredentialFile(cfg.SecretKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read secret key file: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	}
	if httpClient != nil {
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(httpClient))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = cfg.UsePathStyle
		options.Retryer = retry.NewStandard(func(retryOptions *retry.StandardOptions) { retryOptions.MaxAttempts = cfg.RetryMaxAttempts })
	})
	return &S3Store{client: client, bucket: cfg.Bucket, timeout: cfg.Timeout}, nil
}

func (store *S3Store) VerifyReadiness(ctx context.Context) error {
	requestCtx, cancel := store.withTimeout(ctx)
	defer cancel()
	output, err := store.client.HeadObject(requestCtx, &s3.HeadObjectInput{
		Bucket:       aws.String(store.bucket),
		Key:          aws.String(string(readinessSentinelKey)),
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return mapProviderError(err)
	}
	if output == nil || output.ContentLength == nil || *output.ContentLength != readinessSentinelContentLength ||
		output.ChecksumSHA256 == nil || *output.ChecksumSHA256 != readinessSentinelChecksumSHA256 {
		return ErrObjectIntegrity
	}
	digest, ok := metadataDigest(output.Metadata)
	if !ok || digest != readinessSentinelDigest || len(output.Metadata) != 1 {
		return ErrObjectIntegrity
	}
	return nil
}
func normalizeConfig(cfg Config) (Config, error) {
	cfg.Endpoint, cfg.Region, cfg.Bucket = strings.TrimSpace(cfg.Endpoint), strings.TrimSpace(cfg.Region), strings.TrimSpace(cfg.Bucket)
	if cfg.Region == "" {
		return Config{}, errors.New("S3 region is required")
	}
	if cfg.Bucket == "" {
		return Config{}, errors.New("S3 bucket is required")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultRequestTimeout
	}
	if cfg.Timeout < 0 || cfg.Timeout > maxRequestTimeout {
		return Config{}, errors.New("S3 request timeout is out of bounds")
	}
	if cfg.RetryMaxAttempts == 0 {
		cfg.RetryMaxAttempts = defaultMaxAttempts
	}
	if cfg.RetryMaxAttempts < 1 || cfg.RetryMaxAttempts > maxRetryAttempts {
		return Config{}, errors.New("S3 retry attempts are out of bounds")
	}
	return cfg, nil
}
func validateEndpoint(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("S3 endpoint must be an absolute URL without credentials or query parameters")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && parsed.Hostname() == "rustfs" && parsed.Port() == "9000" && (parsed.Path == "" || parsed.Path == "/") {
		return nil
	}
	return errors.New("S3 endpoint must use HTTPS, except http://rustfs:9000")
}
func readCredentialFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("credential file is required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", errors.New("credential file is empty")
	}
	return value, nil
}
func ValidateObjectKey(key ObjectKey) error {
	value := string(key)
	if len(value) != len(objectKeyPrefix)+36 || !strings.HasPrefix(value, objectKeyPrefix) || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
		return fmt.Errorf("%w: %w", ErrInvalidObjectKey, ErrObjectIntegrity)
	}
	id := value[len(objectKeyPrefix):]
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return fmt.Errorf("%w: %w", ErrInvalidObjectKey, ErrObjectIntegrity)
	}
	return nil
}
func (store *S3Store) Put(ctx context.Context, input PutObject) error {
	if err := ValidateObjectKey(input.Key); err != nil {
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
func validateAndRewind(input PutObject) error {
	if input.Body == nil || input.Size < 0 {
		return ErrObjectIntegrity
	}
	if _, err := input.Body.Seek(0, io.SeekStart); err != nil {
		return ErrObjectIntegrity
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
		return ErrObjectIntegrity
	}
	return nil
}
func rewindIntegrity(body io.Seeker) error {
	_, _ = body.Seek(0, io.SeekStart)
	return ErrObjectIntegrity
}
func (store *S3Store) reconcilePut(key ObjectKey, size int64, digest [32]byte) error {
	reconcileCtx, cancel := context.WithTimeout(context.Background(), store.timeout)
	defer cancel()
	output, err := store.client.HeadObject(reconcileCtx, &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	if err != nil {
		mapped := mapProviderError(err)
		if errors.Is(mapped, ErrObjectNotFound) {
			return ErrObjectUnavailable
		}
		return mapped
	}
	if output == nil || output.ContentLength == nil {
		return ErrObjectIntegrity
	}
	metadataDigest, ok := metadataDigest(output.Metadata)
	if !ok || *output.ContentLength != size || metadataDigest != hex.EncodeToString(digest[:]) {
		return ErrObjectExists
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
func (store *S3Store) Open(ctx context.Context, key ObjectKey) (Object, error) {
	if err := ValidateObjectKey(key); err != nil {
		return Object{}, err
	}
	requestCtx, cancel := store.withTimeout(ctx)
	output, err := store.client.GetObject(requestCtx, &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	if err != nil {
		cancel()
		return Object{}, mapProviderError(err)
	}
	if output == nil || output.Body == nil || output.ContentLength == nil {
		closeGetObject(output, cancel)
		return Object{}, ErrObjectIntegrity
	}
	digestText, ok := metadataDigest(output.Metadata)
	if !ok {
		closeGetObject(output, cancel)
		return Object{}, ErrObjectIntegrity
	}
	digestBytes, err := hex.DecodeString(digestText)
	if err != nil {
		closeGetObject(output, cancel)
		return Object{}, ErrObjectIntegrity
	}
	if output.ChecksumSHA256 != nil {
		checksum, checksumErr := base64.StdEncoding.DecodeString(*output.ChecksumSHA256)
		if checksumErr != nil || !bytes.Equal(checksum, digestBytes) {
			closeGetObject(output, cancel)
			return Object{}, ErrObjectIntegrity
		}
	}
	var digest [32]byte
	copy(digest[:], digestBytes)
	return Object{Body: &cancelReadCloser{body: output.Body, cancel: cancel}, Size: *output.ContentLength, SHA256: digest}, nil
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
func (store *S3Store) Delete(ctx context.Context, key ObjectKey) error {
	if err := ValidateObjectKey(key); err != nil {
		return err
	}
	requestCtx, cancel := store.withTimeout(ctx)
	_, err := store.client.DeleteObject(requestCtx, &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(string(key))})
	cancel()
	if err == nil {
		return nil
	}
	mapped := mapProviderError(err)
	if errors.Is(mapped, ErrObjectNotFound) {
		return nil
	}
	return mapped
}
func (store *S3Store) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, store.timeout)
}
func shouldReconcilePut(err error) bool {
	if mapProviderError(err) == ErrObjectExists {
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
			return ErrObjectNotFound
		case "baddigest", "invaliddigest", "checksummismatch", "invalidchecksum":
			return ErrObjectIntegrity
		case "preconditionfailed", "conditionalrequestconflict":
			return ErrObjectExists
		default:
			return ErrObjectUnavailable
		}
	}
	var responseError *smithyhttp.ResponseError
	if errors.As(err, &responseError) {
		switch responseError.HTTPStatusCode() {
		case 400:
			return ErrObjectIntegrity
		case 409, 412:
			return ErrObjectExists
		default:
			return ErrObjectUnavailable
		}
	}
	return ErrObjectUnavailable
}

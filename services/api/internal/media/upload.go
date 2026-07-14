package media

import (
	"context"
	"errors"
	"time"
)

const (
	MaxEncodedImageSize     int64 = 10 << 20
	MaxMultipartBodySize    int64 = MaxEncodedImageSize + 64<<10
	MaxImageWidth                 = 8192
	MaxImageHeight                = 8192
	MaxImageArea                  = 16_000_000
	MaxLiveUploads                = 5
	MaxLiveBytes            int64 = 512 << 20
	UploadLeaseDuration           = 2 * time.Minute
	UploadTTL                     = 24 * time.Hour
	UploadRequestRetention        = 30 * 24 * time.Hour
	DefaultCleanupBatch           = 32
	DefaultOperationTimeout       = 15 * time.Second
	DefaultCleanupTimeout         = 15 * time.Second
)

var (
	ErrUploadNotFound            = errors.New("image upload not found")
	ErrUploadInProgress          = errors.New("image upload is in progress")
	ErrUploadExpired             = errors.New("image upload expired")
	ErrUploadIdempotencyConflict = errors.New("image upload idempotency conflict")
	ErrUploadStateConflict       = errors.New("image upload state conflict")
	ErrUploadQuotaExceeded       = errors.New("image upload staging quota exceeded")
	ErrInvalidUploadRequest      = errors.New("invalid image upload request")
	ErrInvalidMedia              = errors.New("invalid image media")
	ErrUnsupportedMediaType      = errors.New("unsupported image media type")
	ErrMediaTooLarge             = errors.New("image media is too large")
	ErrMediaDimensions           = errors.New("image dimensions exceed limits")
	ErrMediaStorageUnavailable   = errors.New("media storage unavailable")
	ErrMediaIntegrity            = errors.New("media integrity error")
)

type UploadState string

const (
	UploadPending  UploadState = "pending"
	UploadReady    UploadState = "ready"
	UploadConsumed UploadState = "consumed"
	UploadDeleting UploadState = "deleting"
	UploadExpired  UploadState = "expired"
)

type Upload struct {
	ID                    string
	UserID                string
	StorageKey            ObjectKey
	UploadRequestID       string
	State                 UploadState
	ConsumedNoteID        string
	ContentType           string
	ByteSize              int64
	Width                 int
	Height                int
	SHA256                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	WriteLeaseUntil       time.Time
	ExpiresAt             time.Time
	RequestRetentionUntil time.Time
}
type PendingInput struct {
	ID                    string
	UserID                string
	StorageKey            ObjectKey
	UploadRequestID       string
	ContentType           string
	ByteSize              int64
	Width                 int
	Height                int
	SHA256                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	WriteLeaseUntil       time.Time
	ExpiresAt             time.Time
	RequestRetentionUntil time.Time
}
type ReadyInput struct {
	ID              string
	UserID          string
	UploadRequestID string
	SHA256          string
	WriteLeaseUntil time.Time
}
type LeaseInput struct {
	ID              string
	UserID          string
	UploadRequestID string
	SHA256          string
	WriteLeaseUntil time.Time
}
type Quota struct {
	UserCount   int
	GlobalBytes int64
}
type UploadRepository interface {
	FindByUserRequest(context.Context, string, string) (Upload, error)
	BeginPending(context.Context, PendingInput) (Upload, error)
	MarkReady(context.Context, ReadyInput) (bool, error)
	ClearLease(context.Context, LeaseInput) error
	MarkDeleting(context.Context, LeaseInput) error
	ClaimExpired(context.Context, time.Time, int) ([]Upload, error)
	FinalizeExpired(context.Context, string, time.Time) error
	CompactExpired(context.Context, time.Time, int) (int64, error)
	QuotaSnapshot(context.Context, string, time.Time) (Quota, error)
}
type UploadReceipt struct {
	ImageUploadID string
	ContentType   string
	ByteSize      int64
	Width         int
	Height        int
	ExpiresAt     time.Time
}
type RetryAfterError struct {
	Cause error
	After time.Duration
}

func (err *RetryAfterError) Error() string {
	if err == nil || err.Cause == nil {
		return ""
	}
	return err.Cause.Error()
}
func (err *RetryAfterError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}
func RetryAfter(err error) time.Duration {
	var retryErr *RetryAfterError
	if errors.As(err, &retryErr) && retryErr.After > 0 {
		return retryErr.After
	}
	if errors.Is(err, ErrUploadInProgress) {
		return UploadLeaseDuration
	}
	if errors.Is(err, ErrUploadQuotaExceeded) {
		return time.Minute
	}
	if errors.Is(err, ErrMediaStorageUnavailable) {
		return 5 * time.Second
	}
	return 0
}

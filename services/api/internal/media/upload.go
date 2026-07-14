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

// Upload is the durable aggregate for one staged image.
type Upload struct {
	// ID identifies this durable record. UserID and UploadRequestID form a
	// user-scoped idempotency identity with one content payload.
	ID     string
	UserID string
	// StorageKey locates the private object; ContentType, ByteSize, Width,
	// Height, and SHA256 are verified metadata for that object.
	StorageKey      ObjectKey
	UploadRequestID string
	// State tracks lifecycle: Pending awaits or holds object write ownership,
	// Ready can be consumed, Consumed belongs to a note, Deleting is
	// cleanup-owned, and Expired is terminal.
	State UploadState
	// ConsumedNoteID identifies the note that owns a consumed upload.
	ConsumedNoteID string
	ContentType    string
	ByteSize       int64
	Width          int
	Height         int
	SHA256         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// WriteLeaseUntil is the exact conditional-write fence returned by a
	// pending claim. Ready, lease-clear, and deleting transitions require it
	// to remain unexpired and match ID, UserID, UploadRequestID, SHA256, and
	// pending state, so a stale writer cannot mutate a reclaimed upload.
	WriteLeaseUntil time.Time
	// ExpiresAt ends use of a pending or ready upload, normally after 24 hours.
	// RequestRetentionUntil preserves its request identity for 30 days after
	// creation, so retries retain the original outcome rather than make a new upload.
	ExpiresAt             time.Time
	RequestRetentionUntil time.Time
}

// PendingInput is service-owned request metadata for creating or reclaiming a
// pending upload. A duplicate UserID/UploadRequestID must retain the same
// content metadata or it is an idempotency conflict.
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

// ReadyInput is the write owner's proof that object persistence completed. It
// must carry the claimed row's identity, digest, and exact active lease; only
// then can the repository move Pending to Ready.
type ReadyInput struct {
	ID              string
	UserID          string
	UploadRequestID string
	SHA256          string
	WriteLeaseUntil time.Time
}

// LeaseInput is held by the current writer when it releases a failed write or
// transfers it to cleanup. It uses the same identity, digest, and exact lease
// fence as ReadyInput, so stale compensation cannot affect a new claim.
type LeaseInput struct {
	ID              string
	UserID          string
	UploadRequestID string
	SHA256          string
	WriteLeaseUntil time.Time
}

// Quota is the repository-produced admission snapshot. UserCount covers one
// user's live records, while GlobalBytes covers all live records. Pending and
// Ready count only before expiry; Deleting counts until cleanup finalizes it.
type Quota struct {
	UserCount   int
	GlobalBytes int64
}

// UploadRepository owns atomic upload metadata and lifecycle transitions;
// callers own object-store I/O. Its conditional methods preserve user-scoped
// request idempotency and the lease fence across retries and cleanup.
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

// UploadReceipt is the service-produced result after a ready upload is
// persisted or its idempotent outcome is replayed. It exposes media metadata
// and the usability deadline, while the object key and write lease stay private.
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

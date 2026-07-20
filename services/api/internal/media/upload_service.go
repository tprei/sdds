package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"strings"
	"sync"
	"time"
)

// UploadConfig configures upload staging, timing, and cleanup.
type UploadConfig struct {
	// ScratchDir is the directory used for temporary upload bodies; an empty value uses the system temporary directory.
	ScratchDir string
	// Clock supplies lifecycle timestamps; a nil value uses time.Now.
	Clock func() time.Time
	// OperationTimeout bounds background reconciliation and compensation; a non-positive value uses DefaultOperationTimeout.
	OperationTimeout time.Duration
	// CleanupTimeout bounds one CleanupExpired call; a non-positive value uses DefaultCleanupTimeout.
	CleanupTimeout time.Duration
	// CleanupBatch limits rows claimed and compacted per cleanup; a non-positive value uses DefaultCleanupBatch.
	CleanupBatch int
}

// UploadService coordinates image staging and persistence. It owns temporary staged bodies during PrepareImageUpload; callers retain ownership of its repository and object-store dependencies.
type UploadService struct {
	repository UploadRepository
	store      ObjectStore
	config     UploadConfig
}

// UploadReceiver writes one complete image body to writer and returns its canonical lowercase UUID request ID. Retries must write the same body and return the same ID.
type UploadReceiver func(context.Context, io.Writer) (uploadRequestID string, err error)

func NewUploadService(repository UploadRepository, store ObjectStore, config UploadConfig) (*UploadService, error) {
	if repository == nil {
		return nil, errors.New("upload repository is required")
	}
	if store == nil {
		return nil, errors.New("upload object store is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.OperationTimeout <= 0 {
		config.OperationTimeout = DefaultOperationTimeout
	}
	if config.CleanupTimeout <= 0 {
		config.CleanupTimeout = DefaultCleanupTimeout
	}
	if config.CleanupBatch <= 0 {
		config.CleanupBatch = DefaultCleanupBatch
	}
	return &UploadService{repository: repository, store: store, config: config}, nil
}
func (service *UploadService) PrepareImageUpload(ctx context.Context, userID string, receive UploadReceiver) (receipt UploadReceipt, err error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return UploadReceipt{}, err
	}
	if strings.TrimSpace(userID) == "" || receive == nil {
		return UploadReceipt{}, ErrInvalidUploadRequest
	}
	staged, rawID, err := service.spool(ctx, receive)
	if err != nil {
		return UploadReceipt{}, err
	}
	defer func() {
		if cleanupErr := staged.CloseRemove(); cleanupErr != nil {
			err = errors.Join(err, ErrMediaStorageUnavailable, cleanupErr)
		}
	}()
	requestID, err := normalizeRequestID(rawID)
	if err != nil {
		return UploadReceipt{}, err
	}
	metadata, err := inspectImage(ctx, staged.file)
	if err != nil {
		return UploadReceipt{}, err
	}
	metadata.SHA256 = staged.sha256
	now := service.now()
	if err := ctx.Err(); err != nil {
		return UploadReceipt{}, err
	}
	if _, err := staged.file.Seek(0, io.SeekStart); err != nil {
		return UploadReceipt{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind staged image: %w", err))
	}
	existing, findErr := service.repository.FindByUserRequest(ctx, userID, requestID)
	if findErr != nil && !errors.Is(findErr, ErrUploadNotFound) {
		return UploadReceipt{}, service.mapRepositoryError(findErr)
	}
	if findErr == nil {
		if !sameMetadata(existing, metadata) {
			return UploadReceipt{}, ErrUploadIdempotencyConflict
		}
		if replay, replayErr := service.replayExisting(existing, now); replayErr != nil {
			return UploadReceipt{}, replayErr
		} else if replay {
			return receiptFromUpload(existing), nil
		}
	}
	_ = service.CleanupExpired(ctx, now)
	if err := ctx.Err(); err != nil {
		return UploadReceipt{}, err
	}
	now = service.now()
	pending := newPendingInput(existing, userID, requestID, metadata, now)
	claimed, err := service.repository.BeginPending(ctx, pending)
	if err != nil {
		return UploadReceipt{}, service.mapBeginError(err)
	}
	if !sameMetadata(claimed, metadata) {
		return UploadReceipt{}, ErrUploadIdempotencyConflict
	}
	if claimed.State != UploadPending {
		if replay, replayErr := service.replayExisting(claimed, now); replayErr != nil {
			return UploadReceipt{}, replayErr
		} else if replay {
			return receiptFromUpload(claimed), nil
		}
	}
	if claimed.State != UploadPending {
		return UploadReceipt{}, ErrUploadInProgress
	}
	return service.publishPending(ctx, staged, claimed, userID, requestID, metadata)
}

func newPendingInput(existing Upload, userID, requestID string, metadata imageMetadata, now time.Time) PendingInput {
	pending := PendingInput{ID: existing.ID, UserID: userID, StorageKey: existing.StorageKey, UploadRequestID: requestID, ContentType: metadata.ContentType, ByteSize: metadata.ByteSize, Width: metadata.Width, Height: metadata.Height, SHA256: metadata.SHA256, CreatedAt: existing.CreatedAt, UpdatedAt: now, WriteLeaseUntil: now.Add(UploadLeaseDuration), ExpiresAt: existing.ExpiresAt, RequestRetentionUntil: existing.RequestRetentionUntil}
	if pending.ID == "" {
		pending.ID = uuid.NewString()
	}
	if pending.StorageKey == "" {
		pending.StorageKey = ObjectKey("note-images/" + pending.ID)
	}
	if pending.CreatedAt.IsZero() {
		pending.CreatedAt = now
	}
	if pending.ExpiresAt.IsZero() {
		pending.ExpiresAt = pending.CreatedAt.Add(UploadTTL)
	}
	if pending.RequestRetentionUntil.IsZero() {
		pending.RequestRetentionUntil = pending.CreatedAt.Add(UploadRequestRetention)
	}
	return pending
}

func (service *UploadService) publishPending(ctx context.Context, staged *stagedFile, claimed Upload, userID, requestID string, metadata imageMetadata) (UploadReceipt, error) {
	var digest [32]byte
	decoded, err := hex.DecodeString(metadata.SHA256)
	if err != nil || len(decoded) != len(digest) {
		return UploadReceipt{}, ErrMediaIntegrity
	}
	copy(digest[:], decoded)
	if _, err := staged.file.Seek(0, io.SeekStart); err != nil {
		return UploadReceipt{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind staged image: %w", err))
	}
	putErr := service.store.Put(ctx, PutObject{Key: claimed.StorageKey, Body: staged.file, Size: metadata.ByteSize, SHA256: digest, ContentType: metadata.ContentType})
	if putErr != nil {
		if errors.Is(putErr, ErrObjectExists) {
			verifyErr := service.verifyObject(ctx, claimed.StorageKey, metadata)
			if verifyErr == nil {
				return service.reconcileReady(claimed, metadata, putErr)
			}
			if errors.Is(verifyErr, ErrMediaIntegrity) {
				return UploadReceipt{}, service.compensate(claimed, errors.Join(ErrMediaIntegrity, verifyErr))
			}
			return UploadReceipt{}, errors.Join(ErrMediaStorageUnavailable, putErr, verifyErr)
		}
		return UploadReceipt{}, service.handlePutFailure(claimed, putErr)
	}
	readyInput := ReadyInput{ID: claimed.ID, UserID: userID, UploadRequestID: requestID, SHA256: metadata.SHA256, WriteLeaseUntil: claimed.WriteLeaseUntil}
	readyCtx, cancel := service.boundedBackground()
	ready, readyErr := service.repository.MarkReady(readyCtx, readyInput)
	cancel()
	if readyErr != nil || !ready {
		return service.reconcileReady(claimed, metadata, readyErr)
	}
	claimed.State, claimed.ContentType, claimed.ByteSize, claimed.Width, claimed.Height, claimed.SHA256 = UploadReady, metadata.ContentType, metadata.ByteSize, metadata.Width, metadata.Height, metadata.SHA256
	return receiptFromUpload(claimed), nil
}
func (service *UploadService) cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(nonNilContext(ctx), service.config.CleanupTimeout)
}
func (service *UploadService) CleanupExpired(ctx context.Context, now time.Time) error {
	cleanupCtx, cancel := service.cleanupContext(ctx)
	defer cancel()
	if now.IsZero() {
		now = service.now()
	}
	rows, err := service.repository.ClaimExpired(cleanupCtx, now, service.config.CleanupBatch)
	if err != nil {
		return errors.Join(ErrMediaStorageUnavailable, service.mapRepositoryError(err))
	}
	var cleanupErr error
	for _, upload := range rows {
		if err := service.cleanupDelete(cleanupCtx, upload.StorageKey); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if err := service.repository.FinalizeExpired(cleanupCtx, upload.ID, now); err != nil {
			cleanupErr = errors.Join(cleanupErr, service.mapRepositoryError(err))
		}
	}
	if _, err := service.repository.CompactExpired(cleanupCtx, now, service.config.CleanupBatch); err != nil {
		cleanupErr = errors.Join(cleanupErr, service.mapRepositoryError(err))
	}
	if cleanupErr != nil {
		if errors.Is(cleanupErr, ErrMediaIntegrity) || errors.Is(cleanupErr, context.Canceled) || errors.Is(cleanupErr, context.DeadlineExceeded) {
			return cleanupErr
		}
		return errors.Join(ErrMediaStorageUnavailable, cleanupErr)
	}
	return nil
}
func (service *UploadService) cleanupDelete(ctx context.Context, key ObjectKey) error {
	var err error
	for range 3 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err = service.deleteObject(ctx, key)
		if err == nil || !errors.Is(err, ErrMediaStorageUnavailable) {
			return err
		}
	}
	return err
}
func (service *UploadService) replayExisting(upload Upload, now time.Time) (bool, error) {
	switch upload.State {
	case UploadReady, UploadConsumed, UploadPending:
		expires := upload.ExpiresAt
		if upload.State == UploadConsumed {
			expires = upload.RequestRetentionUntil
		}
		if !expires.IsZero() && !expires.After(now) {
			return false, ErrUploadExpired
		}
		if upload.State != UploadPending {
			return true, nil
		}
		if upload.WriteLeaseUntil.After(now) {
			return false, &RetryAfterError{Cause: ErrUploadInProgress, After: upload.WriteLeaseUntil.Sub(now)}
		}
		return false, nil
	case UploadExpired:
		return false, ErrUploadExpired
	case UploadDeleting:
		return false, &RetryAfterError{Cause: ErrUploadInProgress, After: time.Minute}
	default:
		return false, ErrMediaIntegrity
	}
}
func (service *UploadService) mapBeginError(err error) error {
	mapped := service.mapRepositoryError(err)
	if errors.Is(mapped, ErrUploadInProgress) {
		return &RetryAfterError{Cause: ErrUploadInProgress, After: time.Minute}
	}
	return mapped
}
func (service *UploadService) mapRepositoryError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrUploadNotFound) || errors.Is(err, ErrUploadInProgress) || errors.Is(err, ErrUploadExpired) || errors.Is(err, ErrUploadIdempotencyConflict) || errors.Is(err, ErrUploadStateConflict) || errors.Is(err, ErrUploadQuotaExceeded) || errors.Is(err, ErrInvalidUploadRequest) || errors.Is(err, ErrInvalidMedia) || errors.Is(err, ErrUnsupportedMediaType) || errors.Is(err, ErrMediaTooLarge) || errors.Is(err, ErrMediaDimensions) || errors.Is(err, ErrMediaStorageUnavailable) || errors.Is(err, ErrMediaIntegrity) {
		return err
	}
	return errors.Join(ErrMediaStorageUnavailable, err)
}
func (service *UploadService) handlePutFailure(upload Upload, putErr error) error {
	if errors.Is(putErr, ErrObjectIntegrity) {
		return service.compensate(upload, errors.Join(ErrMediaIntegrity, putErr))
	}
	if errors.Is(putErr, ErrObjectExists) {
		return errors.Join(ErrMediaIntegrity, putErr)
	}
	if errors.Is(putErr, ErrObjectUnavailable) || errors.Is(putErr, ErrObjectNotFound) {
		return errors.Join(ErrMediaStorageUnavailable, service.lease(upload, service.repository.ClearLease))
	}
	if errors.Is(putErr, context.Canceled) || errors.Is(putErr, context.DeadlineExceeded) {
		return putErr
	}
	return errors.Join(ErrMediaStorageUnavailable, putErr, service.lease(upload, service.repository.ClearLease))
}
func (service *UploadService) reconcileReady(upload Upload, metadata imageMetadata, readyErr error) (UploadReceipt, error) {
	findCtx, cancel := service.boundedBackground()
	current, findErr := service.repository.FindByUserRequest(findCtx, upload.UserID, upload.UploadRequestID)
	cancel()
	findErr = service.mapRepositoryError(findErr)
	if findErr != nil {
		cause := error(ErrMediaStorageUnavailable)
		if errors.Is(findErr, ErrUploadNotFound) {
			cause = ErrUploadStateConflict
		}
		return UploadReceipt{}, errors.Join(cause, readyErr, findErr)
	}
	if !sameMetadata(current, metadata) {
		return UploadReceipt{}, errors.Join(ErrUploadStateConflict, readyErr)
	}
	switch current.State {
	case UploadReady, UploadConsumed:
		return receiptFromUpload(current), nil
	case UploadPending:
		now := service.now()
		if !current.WriteLeaseUntil.Equal(upload.WriteLeaseUntil) {
			if current.WriteLeaseUntil.After(now) {
				return UploadReceipt{}, &RetryAfterError{Cause: ErrUploadInProgress, After: current.WriteLeaseUntil.Sub(now)}
			}
			return UploadReceipt{}, errors.Join(ErrUploadStateConflict, readyErr)
		}
		readyCtx, readyCancel := service.boundedBackground()
		ready, retryErr := service.repository.MarkReady(readyCtx, ReadyInput{ID: upload.ID, UserID: upload.UserID, UploadRequestID: upload.UploadRequestID, SHA256: metadata.SHA256, WriteLeaseUntil: upload.WriteLeaseUntil})
		readyCancel()
		if retryErr != nil {
			return UploadReceipt{}, errors.Join(service.mapRepositoryError(retryErr), readyErr)
		}
		if !ready {
			return UploadReceipt{}, errors.Join(ErrUploadStateConflict, readyErr)
		}
		current.State, current.WriteLeaseUntil = UploadReady, time.Time{}
		return receiptFromUpload(current), nil
	default:
		return UploadReceipt{}, errors.Join(ErrUploadStateConflict, readyErr)
	}
}
func (service *UploadService) lease(upload Upload, operation func(context.Context, LeaseInput) error) error {
	ctx, cancel := service.boundedBackground()
	defer cancel()
	return service.mapRepositoryError(operation(ctx, LeaseInput{ID: upload.ID, UserID: upload.UserID, UploadRequestID: upload.UploadRequestID, SHA256: upload.SHA256, WriteLeaseUntil: upload.WriteLeaseUntil}))
}
func (service *UploadService) compensate(upload Upload, cause error) error {
	if err := service.lease(upload, service.repository.MarkDeleting); err != nil {
		return errors.Join(cause, err)
	}
	ctx, cancel := service.boundedBackground()
	defer cancel()
	if err := service.deleteObject(ctx, upload.StorageKey); err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(cause, service.mapRepositoryError(service.repository.FinalizeExpired(ctx, upload.ID, service.now())))
}
func (service *UploadService) deleteObject(ctx context.Context, key ObjectKey) error {
	if key == "" {
		return ErrMediaIntegrity
	}
	err := service.store.Delete(ctx, key)
	if errors.Is(err, ErrObjectNotFound) || err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrObjectIntegrity) || errors.Is(err, ErrInvalidObjectKey) {
		return ErrMediaIntegrity
	}
	return errors.Join(ErrMediaStorageUnavailable, err)
}
func (service *UploadService) verifyObject(ctx context.Context, key ObjectKey, metadata imageMetadata) error {
	if key == "" {
		return ErrMediaIntegrity
	}
	object, err := service.store.Open(ctx, key)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, ErrObjectIntegrity) {
			return ErrMediaIntegrity
		}
		return errors.Join(ErrMediaStorageUnavailable, err)
	}
	if object.Body == nil {
		return ErrMediaIntegrity
	}
	if object.Size != metadata.ByteSize {
		return errors.Join(ErrMediaIntegrity, object.Body.Close())
	}
	closeBody := sync.OnceValue(object.Body.Close)
	hasher := sha256.New()
	reader, stop := contextObjectReader(ctx, object.Body, closeBody)
	count, readErr := io.Copy(hasher, reader)
	stop()
	if err := closeBody(); err != nil {
		readErr = errors.Join(readErr, err)
	}
	integrityErr := error(nil)
	if count != metadata.ByteSize || !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), metadata.SHA256) {
		integrityErr = ErrMediaIntegrity
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(ctxErr, readErr, integrityErr)
	}
	if readErr != nil {
		return errors.Join(ErrMediaStorageUnavailable, readErr, integrityErr)
	}
	if integrityErr != nil {
		return integrityErr
	}
	return nil
}
func (service *UploadService) boundedBackground() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), service.config.OperationTimeout)
}
func receiptFromUpload(upload Upload) UploadReceipt {
	return UploadReceipt{ImageUploadID: upload.ID, ContentType: upload.ContentType, ByteSize: upload.ByteSize, Width: upload.Width, Height: upload.Height, ExpiresAt: upload.ExpiresAt}
}
func normalizeRequestID(value string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || parsed.String() != strings.ToLower(strings.TrimSpace(value)) {
		return "", ErrInvalidUploadRequest
	}
	return parsed.String(), nil
}
func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
func (service *UploadService) now() time.Time {
	return service.config.Clock().UTC()
}

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
)

func (store *ImageUploadStore) BeginPending(ctx context.Context, input media.PendingInput) (upload media.Upload, err error) {
	now := normalizeTime(store.clock())
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return media.Upload{}, fmt.Errorf("begin image upload pending transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback image upload pending transaction: %w", rollbackErr)
		}
	}()

	return store.beginPendingInTransaction(ctx, tx, input, now)
}

func (store *ImageUploadStore) beginPendingInTransaction(ctx context.Context, tx *sql.Tx, input media.PendingInput, now time.Time) (media.Upload, error) {
	current, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByUserRequestSQL, input.UserID, input.UploadRequestID))
	switch {
	case errors.Is(err, media.ErrUploadNotFound):
		return store.createPendingImageUpload(ctx, tx, input, now)
	case err != nil:
		return media.Upload{}, fmt.Errorf("find image upload request: %w", err)
	}

	input, err = normalizePendingLease(input, now)
	if err != nil {
		return media.Upload{}, err
	}
	return store.claimExistingPending(ctx, tx, current, input, now)
}

func (store *ImageUploadStore) createPendingImageUpload(ctx context.Context, tx *sql.Tx, input media.PendingInput, now time.Time) (media.Upload, error) {
	input, err := normalizePendingInput(input, now)
	if err != nil {
		return media.Upload{}, err
	}
	if err := store.enforceUploadQuota(ctx, tx, input.UserID, input.ByteSize, now); err != nil {
		return media.Upload{}, err
	}
	if _, err := tx.ExecContext(ctx, insertImageUploadSQL, input.ID, input.UserID, string(input.StorageKey),
		input.UploadRequestID, media.UploadPending, input.ContentType, input.ByteSize, input.Width, input.Height,
		input.SHA256, unixMillis(input.CreatedAt), unixMillis(input.UpdatedAt), unixMillis(input.WriteLeaseUntil),
		unixMillis(input.ExpiresAt), unixMillis(input.RequestRetentionUntil)); err != nil {
		if isUniqueConstraintError(err) {
			current, findErr := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByUserRequestSQL, input.UserID, input.UploadRequestID))
			if findErr == nil {
				return store.claimExistingPending(ctx, tx, current, input, now)
			}
		}
		return media.Upload{}, fmt.Errorf("insert pending image upload: %w", err)
	}

	return store.loadAndCommitCreatedPending(ctx, tx, input.ID)
}

func (store *ImageUploadStore) loadAndCommitCreatedPending(ctx context.Context, tx *sql.Tx, id string) (media.Upload, error) {
	created, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByIDSQL, id))
	if err != nil {
		return media.Upload{}, fmt.Errorf("load pending image upload: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return media.Upload{}, fmt.Errorf("commit pending image upload: %w", err)
	}
	return created, nil
}

func (store *ImageUploadStore) claimExistingPending(ctx context.Context, tx *sql.Tx, current media.Upload, input media.PendingInput, now time.Time) (media.Upload, error) {
	if !sameUploadContent(current, input) {
		return media.Upload{}, media.ErrUploadIdempotencyConflict
	}

	switch current.State {
	case media.UploadPending:
		return store.reclaimPendingUpload(ctx, tx, current, input, now)
	case media.UploadReady:
		return store.replayReadyUpload(tx, current, now)
	case media.UploadConsumed:
		return store.replayConsumedUpload(tx, current, now)
	case media.UploadDeleting:
		return media.Upload{}, media.ErrUploadInProgress
	case media.UploadExpired:
		return media.Upload{}, media.ErrUploadExpired
	default:
		return media.Upload{}, fmt.Errorf("unknown image upload state %q", current.State)
	}
}

func (store *ImageUploadStore) reclaimPendingUpload(ctx context.Context, tx *sql.Tx, current media.Upload, input media.PendingInput, now time.Time) (media.Upload, error) {
	if !current.ExpiresAt.After(now) {
		return media.Upload{}, media.ErrUploadExpired
	}
	if current.WriteLeaseUntil.After(now) {
		return media.Upload{}, media.ErrUploadInProgress
	}

	leaseUntil := input.WriteLeaseUntil
	if !leaseUntil.After(now) {
		leaseUntil = now.Add(media.UploadLeaseDuration)
	}
	if err := store.claimExpiredPendingLease(ctx, tx, current.ID, leaseUntil, now); err != nil {
		return media.Upload{}, err
	}
	return store.loadAndCommitReclaimedPending(ctx, tx, current.ID)
}

func (store *ImageUploadStore) claimExpiredPendingLease(ctx context.Context, tx *sql.Tx, id string, leaseUntil, now time.Time) error {
	result, err := tx.ExecContext(ctx, reclaimPendingImageUploadSQL,
		unixMillis(leaseUntil),
		unixMillis(now),
		id,
		unixMillis(now),
	)
	if err != nil {
		return fmt.Errorf("reclaim pending image upload: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read reclaimed image upload count: %w", err)
	}
	if updated == 0 {
		return media.ErrUploadInProgress
	}
	return nil
}

func (store *ImageUploadStore) loadAndCommitReclaimedPending(ctx context.Context, tx *sql.Tx, id string) (media.Upload, error) {
	claimed, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByIDSQL, id))
	if err != nil {
		return media.Upload{}, fmt.Errorf("load reclaimed image upload: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return media.Upload{}, fmt.Errorf("commit reclaimed image upload: %w", err)
	}
	return claimed, nil
}

func (store *ImageUploadStore) replayReadyUpload(tx *sql.Tx, current media.Upload, now time.Time) (media.Upload, error) {
	if !current.ExpiresAt.After(now) {
		return media.Upload{}, media.ErrUploadExpired
	}
	if err := tx.Commit(); err != nil {
		return media.Upload{}, fmt.Errorf("commit existing image upload: %w", err)
	}
	return current, nil
}

func (store *ImageUploadStore) replayConsumedUpload(tx *sql.Tx, current media.Upload, now time.Time) (media.Upload, error) {
	if !current.RequestRetentionUntil.After(now) {
		return media.Upload{}, media.ErrUploadExpired
	}
	if err := tx.Commit(); err != nil {
		return media.Upload{}, fmt.Errorf("commit existing image upload: %w", err)
	}
	return current, nil
}

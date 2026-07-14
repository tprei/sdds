package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/tprei/sdds/services/api/internal/media"
	"time"
)

const (
	imageUploadColumns = `
		id,
		user_id,
		storage_key,
		upload_request_id,
		state,
		consumed_note_id,
		content_type,
		byte_size,
		width,
		height,
		sha256,
		created_at,
		updated_at,
		write_lease_until,
		expires_at,
		request_retention_until`
	findImageUploadByUserRequestSQL = `SELECT` + imageUploadColumns + `
		FROM image_uploads
		WHERE user_id = ? AND upload_request_id = ?`
	findImageUploadByIDSQL = `SELECT` + imageUploadColumns + `
		FROM image_uploads
		WHERE id = ?`
	insertImageUploadSQL = `
		INSERT INTO image_uploads (
			id, user_id, storage_key, upload_request_id, state, consumed_note_id,
			content_type, byte_size, width, height, sha256, created_at, updated_at,
			write_lease_until, expires_at, request_retention_until
		) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	liveImageUploadQuotaSQL = `
		SELECT
			COALESCE(SUM(CASE WHEN user_id = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(byte_size), 0)
		FROM image_uploads
		WHERE state IN ('pending', 'ready', 'deleting')`
	claimImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'deleting', write_lease_until = ?, updated_at = ?
		WHERE id = ? AND (
			(state IN ('pending', 'ready') AND expires_at <= ?) OR
			(state = 'deleting' AND (write_lease_until IS NULL OR write_lease_until <= ?)))`
	markImageUploadReadySQL = `
		UPDATE image_uploads
		SET state = 'ready', write_lease_until = NULL, updated_at = ?
		WHERE id = ? AND user_id = ? AND upload_request_id = ? AND sha256 = ?
			AND state = 'pending' AND write_lease_until = ? AND write_lease_until > ?`
	clearImageUploadLeaseSQL = `
		UPDATE image_uploads
		SET write_lease_until = NULL, updated_at = ?
		WHERE id = ? AND user_id = ? AND upload_request_id = ? AND sha256 = ?
			AND state = 'pending' AND write_lease_until = ? AND write_lease_until > ?`
	markImageUploadDeletingSQL = `
		UPDATE image_uploads
		SET state = 'deleting', write_lease_until = NULL, updated_at = ?
		WHERE id = ? AND user_id = ? AND upload_request_id = ? AND sha256 = ?
			AND state = 'pending' AND write_lease_until = ? AND write_lease_until > ?`
	finalizeImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'expired', write_lease_until = NULL, updated_at = ?
		WHERE id = ? AND state = 'deleting'`
)

// ImageUploadStore persists private staged-upload lifecycle metadata.
type ImageUploadStore struct {
	db    *sql.DB
	clock func() time.Time
}

var _ media.UploadRepository = (*ImageUploadStore)(nil)

func NewImageUploadStore(db *sql.DB) *ImageUploadStore {
	return newImageUploadStore(db, time.Now)
}
func newImageUploadStore(db *sql.DB, clock func() time.Time) *ImageUploadStore {
	return &ImageUploadStore{db: db, clock: clock}
}
func (store *ImageUploadStore) FindByUserRequest(ctx context.Context, userID, requestID string) (media.Upload, error) {
	return store.findUpload(ctx, store.db.QueryRowContext(ctx, findImageUploadByUserRequestSQL, userID, requestID))
}
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
	current, findErr := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByUserRequestSQL, input.UserID, input.UploadRequestID))
	if errors.Is(findErr, media.ErrUploadNotFound) {
		input, err = normalizePendingInput(input, now)
		if err != nil {
			return media.Upload{}, err
		}
		if err := store.enforceUploadQuota(ctx, tx, input.UserID, input.ByteSize); err != nil {
			return media.Upload{}, err
		}
		if _, err := tx.ExecContext(ctx, insertImageUploadSQL, input.ID, input.UserID, string(input.StorageKey),
			input.UploadRequestID, media.UploadPending, input.ContentType, input.ByteSize, input.Width, input.Height,
			input.SHA256, unixMillis(input.CreatedAt), unixMillis(input.UpdatedAt), unixMillis(input.WriteLeaseUntil),
			unixMillis(input.ExpiresAt), unixMillis(input.RequestRetentionUntil)); err != nil {
			if isUniqueConstraintError(err) {
				current, findErr = store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByUserRequestSQL, input.UserID, input.UploadRequestID))
				if findErr == nil {
					return store.claimExistingPending(ctx, tx, current, input, now)
				}
			}
			return media.Upload{}, fmt.Errorf("insert pending image upload: %w", err)
		}
		created, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByIDSQL, input.ID))
		if err != nil {
			return media.Upload{}, fmt.Errorf("load pending image upload: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return media.Upload{}, fmt.Errorf("commit pending image upload: %w", err)
		}
		return created, nil
	}
	if findErr != nil {
		return media.Upload{}, fmt.Errorf("find image upload request: %w", findErr)
	}
	input, err = normalizePendingLease(input, now)
	if err != nil {
		return media.Upload{}, err
	}
	return store.claimExistingPending(ctx, tx, current, input, now)
}
func (store *ImageUploadStore) claimExistingPending(ctx context.Context, tx *sql.Tx, current media.Upload, input media.PendingInput, now time.Time) (media.Upload, error) {
	if !sameUploadContent(current, input) {
		return media.Upload{}, media.ErrUploadIdempotencyConflict
	}
	switch current.State {
	case media.UploadPending:
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
		result, err := tx.ExecContext(
			ctx,
			`UPDATE image_uploads SET write_lease_until = ?, updated_at = ? WHERE id = ? AND state = 'pending' AND (write_lease_until IS NULL OR write_lease_until <= ?)`,
			unixMillis(leaseUntil),
			unixMillis(now),
			current.ID,
			unixMillis(now),
		)
		if err != nil {
			return media.Upload{}, fmt.Errorf("reclaim pending image upload: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return media.Upload{}, fmt.Errorf("read reclaimed image upload count: %w", err)
		}
		if updated == 0 {
			return media.Upload{}, media.ErrUploadInProgress
		}
		claimed, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByIDSQL, current.ID))
		if err != nil {
			return media.Upload{}, fmt.Errorf("load reclaimed image upload: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return media.Upload{}, fmt.Errorf("commit reclaimed image upload: %w", err)
		}
		return claimed, nil
	case media.UploadReady:
		if !current.ExpiresAt.After(now) {
			return media.Upload{}, media.ErrUploadExpired
		}
		if err := tx.Commit(); err != nil {
			return media.Upload{}, fmt.Errorf("commit existing image upload: %w", err)
		}
		return current, nil
	case media.UploadConsumed:
		if !current.RequestRetentionUntil.After(now) {
			return media.Upload{}, media.ErrUploadExpired
		}
		if err := tx.Commit(); err != nil {
			return media.Upload{}, fmt.Errorf("commit existing image upload: %w", err)
		}
		return current, nil
	case media.UploadDeleting:
		return media.Upload{}, media.ErrUploadInProgress
	case media.UploadExpired:
		return media.Upload{}, media.ErrUploadExpired
	default:
		return media.Upload{}, fmt.Errorf("unknown image upload state %q", current.State)
	}
}
func (store *ImageUploadStore) MarkReady(ctx context.Context, input media.ReadyInput) (bool, error) {
	now := normalizeTime(store.clock())
	lease := normalizeTime(input.WriteLeaseUntil)
	if !validLease(lease, now) {
		return false, nil
	}
	result, err := store.db.ExecContext(ctx, markImageUploadReadySQL, unixMillis(now),
		input.ID, input.UserID, input.UploadRequestID, input.SHA256, unixMillis(lease), unixMillis(now))
	if err != nil {
		return false, fmt.Errorf("mark image upload ready: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read marked image upload count: %w", err)
	}
	return updated == 1, nil
}
func (store *ImageUploadStore) ClearLease(ctx context.Context, input media.LeaseInput) error {
	return store.updateLease(ctx, clearImageUploadLeaseSQL, input, "clear image upload lease")
}
func (store *ImageUploadStore) MarkDeleting(ctx context.Context, input media.LeaseInput) error {
	return store.updateLease(ctx, markImageUploadDeletingSQL, input, "mark image upload deleting")
}
func (store *ImageUploadStore) updateLease(ctx context.Context, query string, input media.LeaseInput, operation string) error {
	now := normalizeTime(store.clock())
	lease := normalizeTime(input.WriteLeaseUntil)
	if !validLease(lease, now) {
		return fmt.Errorf("%s: %w", operation, media.ErrUploadStateConflict)
	}
	result, err := store.db.ExecContext(ctx, query, unixMillis(now),
		input.ID, input.UserID, input.UploadRequestID, input.SHA256, unixMillis(lease), unixMillis(now))
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s count: %w", operation, err)
	}
	if updated != 1 {
		return fmt.Errorf("%s: %w", operation, media.ErrUploadStateConflict)
	}
	return nil
}
func (store *ImageUploadStore) ClaimExpired(ctx context.Context, now time.Time, limit int) (uploads []media.Upload, err error) {
	if limit <= 0 {
		return []media.Upload{}, nil
	}
	if now.IsZero() {
		now = store.clock()
	}
	now = normalizeTime(now)
	leaseUntil := normalizeTime(now.Add(media.UploadLeaseDuration))
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin image upload cleanup claim: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback image upload cleanup claim: %w", rollbackErr)
		}
	}()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM image_uploads
		WHERE (state IN ('pending', 'ready') AND expires_at <= ?)
		   OR (state = 'deleting' AND (write_lease_until IS NULL OR write_lease_until <= ?))
		ORDER BY CASE WHEN state = 'deleting' THEN updated_at ELSE expires_at END, id LIMIT ?`,
		unixMillis(now), unixMillis(now), limit)
	if err != nil {
		return nil, fmt.Errorf("select expired image uploads: %w", err)
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan expired image upload: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read expired image uploads: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close expired image uploads: %w", err)
	}
	uploads = make([]media.Upload, 0, len(ids))
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, claimImageUploadSQL,
			unixMillis(leaseUntil), unixMillis(now), id, unixMillis(now), unixMillis(now))
		if err != nil {
			return nil, fmt.Errorf("claim expired image upload: %w", err)
		}
		claimed, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("read claimed image upload count: %w", err)
		}
		if claimed != 1 {
			continue
		}
		upload, err := store.findUpload(ctx, tx.QueryRowContext(ctx, findImageUploadByIDSQL, id))
		if err != nil {
			return nil, fmt.Errorf("load claimed image upload: %w", err)
		}
		uploads = append(uploads, upload)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit image upload cleanup claim: %w", err)
	}
	return uploads, nil
}
func (store *ImageUploadStore) FinalizeExpired(ctx context.Context, id string, now time.Time) error {
	now = normalizeTime(now)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin image upload cleanup finalize: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, finalizeImageUploadSQL, unixMillis(now), id)
	if err != nil {
		return fmt.Errorf("finalize expired image upload: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read finalized image upload count: %w", err)
	}
	if updated == 0 {
		var state media.UploadState
		err := tx.QueryRowContext(ctx, `SELECT state FROM image_uploads WHERE id = ?`, id).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) {
			return media.ErrUploadNotFound
		}
		if err != nil {
			return fmt.Errorf("check image upload cleanup state: %w", err)
		}
		if state == media.UploadExpired {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit idempotent image upload cleanup: %w", err)
			}
			return nil
		}
		return fmt.Errorf("image upload %s is in state %q", id, state)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit image upload cleanup finalize: %w", err)
	}
	return nil
}
func (store *ImageUploadStore) QuotaSnapshot(ctx context.Context, userID string, now time.Time) (media.Quota, error) {
	_ = now
	var quota media.Quota
	if err := store.db.QueryRowContext(ctx, liveImageUploadQuotaSQL, userID).Scan(&quota.UserCount, &quota.GlobalBytes); err != nil {
		return media.Quota{}, fmt.Errorf("read image upload quota: %w", err)
	}
	return quota, nil
}
func (store *ImageUploadStore) CompactExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = store.clock()
	}
	now = normalizeTime(now)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin image upload retention compaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM image_uploads
		WHERE state IN ('consumed', 'expired') AND request_retention_until <= ?
		ORDER BY request_retention_until ASC, id ASC LIMIT ?`, unixMillis(now), limit)
	if err != nil {
		return 0, fmt.Errorf("select retained image uploads: %w", err)
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan retained image upload: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("read retained image uploads: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close retained image uploads: %w", err)
	}
	var removed int64
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `
			DELETE FROM image_uploads
			WHERE id = ? AND state IN ('consumed', 'expired') AND request_retention_until <= ?`,
			id, unixMillis(now))
		if err != nil {
			return 0, fmt.Errorf("compact expired image upload: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("read compacted image upload count: %w", err)
		}
		removed += count
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit image upload retention compaction: %w", err)
	}
	return removed, nil
}
func (store *ImageUploadStore) enforceUploadQuota(ctx context.Context, tx *sql.Tx, userID string, byteSize int64) error {
	var quota media.Quota
	if err := tx.QueryRowContext(ctx, liveImageUploadQuotaSQL, userID).Scan(&quota.UserCount, &quota.GlobalBytes); err != nil {
		return fmt.Errorf("read image upload quota before insert: %w", err)
	}
	if quota.UserCount >= media.MaxLiveUploads || quota.GlobalBytes > media.MaxLiveBytes-byteSize {
		return media.ErrUploadQuotaExceeded
	}
	return nil
}
func (store *ImageUploadStore) findUpload(ctx context.Context, row interface{ Scan(...any) error }) (media.Upload, error) {
	var (
		upload                media.Upload
		storageKey            string
		state                 string
		consumedNoteID        sql.NullString
		createdAt             int64
		updatedAt             int64
		writeLeaseUntil       sql.NullInt64
		expiresAt             int64
		requestRetentionUntil int64
	)
	if err := row.Scan(
		&upload.ID,
		&upload.UserID,
		&storageKey,
		&upload.UploadRequestID,
		&state,
		&consumedNoteID,
		&upload.ContentType,
		&upload.ByteSize,
		&upload.Width,
		&upload.Height,
		&upload.SHA256,
		&createdAt,
		&updatedAt,
		&writeLeaseUntil,
		&expiresAt,
		&requestRetentionUntil,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Upload{}, media.ErrUploadNotFound
		}
		return media.Upload{}, err
	}
	upload.StorageKey = media.ObjectKey(storageKey)
	upload.State = media.UploadState(state)
	upload.ConsumedNoteID = consumedNoteID.String
	upload.CreatedAt = timeFromUnixMillis(createdAt)
	upload.UpdatedAt = timeFromUnixMillis(updatedAt)
	if writeLeaseUntil.Valid {
		upload.WriteLeaseUntil = timeFromUnixMillis(writeLeaseUntil.Int64)
	}
	upload.ExpiresAt = timeFromUnixMillis(expiresAt)
	upload.RequestRetentionUntil = timeFromUnixMillis(requestRetentionUntil)
	return upload, nil
}
func normalizePendingInput(input media.PendingInput, now time.Time) (media.PendingInput, error) {
	now = normalizeTime(now)
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.CreatedAt = normalizeTime(input.CreatedAt)
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = input.CreatedAt
	}
	input.UpdatedAt = normalizeTime(input.UpdatedAt)
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = input.CreatedAt.Add(media.UploadTTL)
	}
	input.ExpiresAt = normalizeTime(input.ExpiresAt)
	if input.RequestRetentionUntil.IsZero() {
		input.RequestRetentionUntil = input.CreatedAt.Add(media.UploadRequestRetention)
	}
	input.RequestRetentionUntil = normalizeTime(input.RequestRetentionUntil)
	if input.CreatedAt.After(now) || input.UpdatedAt.Before(input.CreatedAt) || input.UpdatedAt.After(now) ||
		!input.ExpiresAt.After(now) || !input.ExpiresAt.After(input.CreatedAt) ||
		!input.RequestRetentionUntil.After(input.ExpiresAt) {
		return media.PendingInput{}, media.ErrInvalidUploadRequest
	}
	return normalizePendingLease(input, now)
}
func normalizePendingLease(input media.PendingInput, now time.Time) (media.PendingInput, error) {
	now = normalizeTime(now)
	if input.WriteLeaseUntil.IsZero() {
		input.WriteLeaseUntil = now.Add(media.UploadLeaseDuration)
	} else {
		input.WriteLeaseUntil = normalizeTime(input.WriteLeaseUntil)
		if !input.WriteLeaseUntil.After(now) {
			return media.PendingInput{}, media.ErrInvalidUploadRequest
		}
	}
	if !input.WriteLeaseUntil.After(input.UpdatedAt) {
		return media.PendingInput{}, media.ErrInvalidUploadRequest
	}
	return input, nil
}
func validLease(leaseUntil, now time.Time) bool {
	return !leaseUntil.IsZero() && leaseUntil.After(now)
}
func sameUploadContent(current media.Upload, input media.PendingInput) bool {
	return current.ContentType == input.ContentType &&
		current.ByteSize == input.ByteSize &&
		current.Width == input.Width &&
		current.Height == input.Height &&
		current.SHA256 == input.SHA256
}

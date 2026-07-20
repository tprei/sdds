package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tprei/sdds/services/api/internal/media"
	"time"
)

const (
	insertImageUploadSQL = `
		INSERT INTO image_uploads (
			id, user_id, storage_key, upload_request_id, state, consumed_note_id,
			content_type, byte_size, width, height, sha256, created_at, updated_at,
			write_lease_until, expires_at, request_retention_until
		) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
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
	reclaimPendingImageUploadSQL = `UPDATE image_uploads SET write_lease_until = ?, updated_at = ? WHERE id = ? AND state = 'pending' AND (write_lease_until IS NULL OR write_lease_until <= ?)`
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

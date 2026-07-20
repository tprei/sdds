package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tprei/sdds/services/api/internal/media"
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
)

func (store *ImageUploadStore) FindByUserRequest(ctx context.Context, userID, requestID string) (media.Upload, error) {
	return store.findUpload(ctx, store.db.QueryRowContext(ctx, findImageUploadByUserRequestSQL, userID, requestID))
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

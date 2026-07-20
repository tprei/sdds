package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	consumeImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'consumed', consumed_note_id = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND state = 'ready' AND consumed_note_id IS NULL AND expires_at > ?
	`
	findImageUploadForAssociationSQL = `
		SELECT id, user_id, storage_key, state, consumed_note_id,
			content_type, byte_size, width, height, sha256,
			created_at, updated_at, expires_at
		FROM image_uploads
		WHERE id = ?
	`
	insertNoteImageSQL = `
		INSERT INTO note_images (
			id, note_id, storage_key, content_type, byte_size, width, height,
			sha256, position, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
)

type noteImageAssociation struct {
	ID          string
	UserID      string
	StorageKey  string
	State       string
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
	SHA256      string
	Position    int
	CreatedAt   int64
	UpdatedAt   int64
	ExpiresAt   int64
}

func associateImageUploads(ctx context.Context, tx *sql.Tx, input note.CreateInput, noteID string, now time.Time) error {
	for position, imageUploadID := range input.ImageUploadIDs {
		upload, err := consumeImageUpload(ctx, tx, string(input.UserID), imageUploadID, noteID, now)
		if err != nil {
			return fmt.Errorf("consume image upload %q: %w", imageUploadID, err)
		}
		upload.Position = position
		if _, err := tx.ExecContext(
			ctx,
			insertNoteImageSQL,
			upload.ID,
			noteID,
			upload.StorageKey,
			upload.ContentType,
			upload.ByteSize,
			upload.Width,
			upload.Height,
			upload.SHA256,
			upload.Position,
			unixMillis(now),
			unixMillis(now),
		); err != nil {
			return fmt.Errorf("insert note image: %w", err)
		}
	}
	return nil
}

func consumeImageUpload(ctx context.Context, tx *sql.Tx, userID, imageUploadID, noteID string, now time.Time) (noteImageAssociation, error) {
	result, err := tx.ExecContext(
		ctx,
		consumeImageUploadSQL,
		noteID,
		unixMillis(now),
		imageUploadID,
		userID,
		unixMillis(now),
	)
	if err != nil {
		return noteImageAssociation{}, fmt.Errorf("transition image upload: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return noteImageAssociation{}, fmt.Errorf("read image upload transition count: %w", err)
	}
	if updated != 1 {
		upload, found, err := readImageUploadForAssociation(ctx, tx, imageUploadID)
		if err != nil {
			return noteImageAssociation{}, fmt.Errorf("read unavailable image upload: %w", err)
		}
		if !found || upload.UserID != userID {
			return noteImageAssociation{}, note.ErrImageUploadUnavailable
		}
		if upload.State == "expired" || (upload.State == "ready" && upload.ExpiresAt <= unixMillis(now)) {
			return noteImageAssociation{}, note.ErrImageUploadExpired
		}
		return noteImageAssociation{}, note.ErrImageUploadUnavailable
	}

	upload, found, err := readImageUploadForAssociation(ctx, tx, imageUploadID)
	if err != nil {
		return noteImageAssociation{}, fmt.Errorf("read consumed image upload: %w", err)
	}
	if !found || upload.UserID != userID {
		return noteImageAssociation{}, note.ErrImageUploadUnavailable
	}
	return upload, nil
}

func readImageUploadForAssociation(ctx context.Context, tx *sql.Tx, imageUploadID string) (noteImageAssociation, bool, error) {
	var upload noteImageAssociation
	var consumedNoteID sql.NullString
	if err := tx.QueryRowContext(ctx, findImageUploadForAssociationSQL, imageUploadID).Scan(
		&upload.ID,
		&upload.UserID,
		&upload.StorageKey,
		&upload.State,
		&consumedNoteID,
		&upload.ContentType,
		&upload.ByteSize,
		&upload.Width,
		&upload.Height,
		&upload.SHA256,
		&upload.CreatedAt,
		&upload.UpdatedAt,
		&upload.ExpiresAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return noteImageAssociation{}, false, nil
		}
		return noteImageAssociation{}, false, err
	}
	return upload, true, nil
}

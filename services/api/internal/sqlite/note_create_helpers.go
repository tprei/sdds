package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

type noteCreateQueryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

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

func noteCreateFingerprint(input note.CreateInput) string {
	hasher := sha256.New()
	var length [8]byte
	writeFrame := func(value []byte) {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hasher.Write(length[:])
		_, _ = hasher.Write(value)
	}
	writeUint64 := func(value uint64) {
		binary.BigEndian.PutUint64(length[:], value)
		_, _ = hasher.Write(length[:])
	}

	writeFrame([]byte("note.create.v1"))
	writeFrame([]byte(input.Title))
	writeFrame([]byte(input.Body))
	writeFrame([]byte(input.CategorySlug))
	if input.PlaceSlug == "" {
		_, _ = hasher.Write([]byte{0})
	} else {
		_, _ = hasher.Write([]byte{1})
		writeFrame([]byte(input.PlaceSlug))
	}
	writeUint64(uint64(len(input.ImageUploadIDs)))
	for _, imageUploadID := range input.ImageUploadIDs {
		writeFrame([]byte(imageUploadID))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func readNoteCreateRequest(ctx context.Context, queryer noteCreateQueryer, userID, clientRequestID string) (requestSHA256, noteID string, found bool, err error) {
	err = queryer.QueryRowContext(ctx, findNoteCreateRequestSQL, userID, clientRequestID).Scan(&requestSHA256, &noteID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return requestSHA256, noteID, true, nil
}

func (store *NoteStore) loadNoteForCreate(ctx context.Context, queryer noteCreateQueryer, id string) (note.Note, error) {
	found, err := scanNoteRow(queryer.QueryRowContext(ctx, findNoteSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return note.Note{}, note.ErrNoteNotFound
		}
		return note.Note{}, err
	}
	notes := []note.Note{found}
	if err := hydrateNoteImagesWithQueryer(ctx, queryer, notes); err != nil {
		return note.Note{}, err
	}
	return notes[0], nil
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

func readImageUploadForAssociation(ctx context.Context, queryer noteCreateQueryer, imageUploadID string) (noteImageAssociation, bool, error) {
	var upload noteImageAssociation
	var consumedNoteID sql.NullString
	if err := queryer.QueryRowContext(ctx, findImageUploadForAssociationSQL, imageUploadID).Scan(
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

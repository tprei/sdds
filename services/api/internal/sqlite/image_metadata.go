package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/media"
)

const findAttachedImageSQL = `
	SELECT id, storage_key, content_type, byte_size, sha256
	FROM note_images
	WHERE id = ?
`

func (store *NoteStore) FindAttachedImage(ctx context.Context, imageID string) (media.ImageMetadata, error) {
	var metadata media.ImageMetadata
	var storageKey string
	var digest string
	if err := store.db.QueryRowContext(ctx, findAttachedImageSQL, imageID).Scan(
		&metadata.ID,
		&storageKey,
		&metadata.ContentType,
		&metadata.ByteSize,
		&digest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.ImageMetadata{}, media.ErrImageNotFound
		}
		return media.ImageMetadata{}, fmt.Errorf("find attached image: %w", err)
	}

	if len(digest) != hex.EncodedLen(sha256.Size) {
		return media.ImageMetadata{}, media.ErrMediaIntegrity
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		return media.ImageMetadata{}, media.ErrMediaIntegrity
	}
	copy(metadata.SHA256[:], decoded)
	metadata.StorageKey = media.ObjectKey(storageKey)
	return metadata, nil
}

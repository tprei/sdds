package sqlite

import (
	"context"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/media"
)

const (
	claimOrphanedMediaObjectsSQL = `
		SELECT storage_key FROM orphaned_media_objects
		ORDER BY orphaned_at, storage_key LIMIT ?`
	forgetOrphanedMediaObjectSQL = `DELETE FROM orphaned_media_objects WHERE storage_key = ?`
)

// ClaimOrphanedObjects returns up to limit storage keys queued for bucket
// deletion, oldest first. The claim is not leased: the upload cleanup sweep is
// single-instance and object deletion is idempotent, so a failed or repeated
// delete simply retries on the next sweep with the row still present.
func (store *ImageUploadStore) ClaimOrphanedObjects(ctx context.Context, limit int) (keys []media.ObjectKey, err error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := store.db.QueryContext(ctx, claimOrphanedMediaObjectsSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("claim orphaned media objects: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close orphaned media rows: %w", closeErr)
		}
	}()
	keys = make([]media.ObjectKey, 0, limit)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan orphaned media object: %w", err)
		}
		keys = append(keys, media.ObjectKey(key))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orphaned media objects: %w", err)
	}
	return keys, nil
}

// ForgetOrphanedObject drops one queued key after its bucket object was
// deleted (or was already absent). A row that vanished between claim and
// forget is not an error.
func (store *ImageUploadStore) ForgetOrphanedObject(ctx context.Context, key media.ObjectKey) error {
	if _, err := store.db.ExecContext(ctx, forgetOrphanedMediaObjectSQL, string(key)); err != nil {
		return fmt.Errorf("forget orphaned media object: %w", err)
	}
	return nil
}

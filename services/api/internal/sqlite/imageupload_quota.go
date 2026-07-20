package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tprei/sdds/services/api/internal/media"
	"time"
)

const liveImageUploadQuotaSQL = `
		SELECT
			COALESCE(SUM(CASE WHEN user_id = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(byte_size), 0)
		FROM image_uploads
		WHERE state IN ('pending', 'ready', 'deleting')
			AND (state = 'deleting' OR expires_at > ?)`

func (store *ImageUploadStore) QuotaSnapshot(ctx context.Context, userID string, now time.Time) (media.Quota, error) {
	now = normalizeTime(now)
	var quota media.Quota
	if err := store.db.QueryRowContext(ctx, liveImageUploadQuotaSQL, userID, unixMillis(now)).Scan(&quota.UserCount, &quota.GlobalBytes); err != nil {
		return media.Quota{}, fmt.Errorf("read image upload quota: %w", err)
	}
	return quota, nil
}
func (store *ImageUploadStore) enforceUploadQuota(ctx context.Context, tx *sql.Tx, userID string, byteSize int64, now time.Time) error {
	now = normalizeTime(now)
	var quota media.Quota
	if err := tx.QueryRowContext(ctx, liveImageUploadQuotaSQL, userID, unixMillis(now)).Scan(&quota.UserCount, &quota.GlobalBytes); err != nil {
		return fmt.Errorf("read image upload quota before insert: %w", err)
	}
	if quota.UserCount >= media.MaxLiveUploads || quota.GlobalBytes > media.MaxLiveBytes-byteSize {
		return media.ErrUploadQuotaExceeded
	}
	return nil
}

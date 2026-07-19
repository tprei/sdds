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
	selectExpiredImageUploadIDsSQL = `
		SELECT id FROM image_uploads
		WHERE (state IN ('pending', 'ready') AND expires_at <= ?)
		   OR (state = 'deleting' AND (write_lease_until IS NULL OR write_lease_until <= ?))
		ORDER BY CASE WHEN state = 'deleting' THEN updated_at ELSE expires_at END, id LIMIT ?`
	claimImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'deleting', write_lease_until = ?, updated_at = ?
		WHERE id = ? AND (
			(state IN ('pending', 'ready') AND expires_at <= ?) OR
			(state = 'deleting' AND (write_lease_until IS NULL OR write_lease_until <= ?)))`
	finalizeImageUploadSQL = `
		UPDATE image_uploads
		SET state = 'expired', write_lease_until = NULL, updated_at = ?
		WHERE id = ? AND state = 'deleting'`
	findImageUploadStateSQL         = `SELECT state FROM image_uploads WHERE id = ?`
	selectRetainedImageUploadIDsSQL = `
		SELECT id FROM image_uploads
		WHERE state IN ('consumed', 'expired') AND request_retention_until <= ?
		ORDER BY request_retention_until ASC, id ASC LIMIT ?`
	deleteRetainedImageUploadSQL = `
			DELETE FROM image_uploads
			WHERE id = ? AND state IN ('consumed', 'expired') AND request_retention_until <= ?`
)

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
	rows, err := tx.QueryContext(ctx, selectExpiredImageUploadIDsSQL, unixMillis(now), unixMillis(now), limit)
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
		err := tx.QueryRowContext(ctx, findImageUploadStateSQL, id).Scan(&state)
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
	rows, err := tx.QueryContext(ctx, selectRetainedImageUploadIDsSQL, unixMillis(now), limit)
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
		result, err := tx.ExecContext(ctx, deleteRetainedImageUploadSQL, id, unixMillis(now))
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

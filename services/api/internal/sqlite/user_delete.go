package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	// note_search is an FTS5 virtual table with no foreign key, so it never
	// cascades through a notes delete and must be cleared explicitly.
	deleteUserNoteSearchSQL = `DELETE FROM note_search WHERE note_id IN (SELECT id FROM notes WHERE user_id = ?)`
	// Carry the user's image storage keys forward inside the same transaction
	// that removes the image_uploads rows. The upload cleanup sweep drains this
	// queue and deletes the bucket bytes; without it the keys would be lost
	// with the rows and the bytes would leak.
	queueUserOrphanedMediaSQL = `
		INSERT OR IGNORE INTO orphaned_media_objects (storage_key, orphaned_at)
		SELECT storage_key, ? FROM image_uploads WHERE user_id = ?`
	deleteUserImageUploadsSQL    = `DELETE FROM image_uploads WHERE user_id = ?`
	deleteUserNotesSQL           = `DELETE FROM notes WHERE user_id = ?`
	deleteUserAuthorsSQL         = `DELETE FROM authors WHERE user_id = ?`
	deleteUserLoginIdentitiesSQL = `DELETE FROM user_login_identities WHERE user_id = ?`
	deleteUserSessionsSQL        = `DELETE FROM sessions WHERE user_id = ?`
	deleteUserSQL                = `DELETE FROM users WHERE id = ?`
)

// DeleteUser permanently removes one user and every personal row tied to it in
// a single transaction. note_search is cleared explicitly (FTS5 carries no
// foreign key); image_uploads, authors, user_login_identities, sessions, and
// notes are deleted explicitly because their user_id foreign key has no ON
// DELETE CASCADE; the remaining tables (note_images, note_embeddings,
// note_comments and their replies, note_useful_reactions, note_create_requests,
// reports, events, user_contact_channels and its tokens) cascade through the
// notes and users deletes under PRAGMA foreign_keys = ON. Image object keys are
// moved to orphaned_media_objects so the upload cleanup sweep deletes the bytes
// from the private bucket and retries until it succeeds. A mid-transaction
// failure rolls back and leaves the account fully intact.
func (store *UserStore) DeleteUser(ctx context.Context, userID user.UserID, deletedAt time.Time) (err error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete user: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			rollbackErr = fmt.Errorf("rollback delete user: %w", rollbackErr)
			if err == nil {
				err = rollbackErr
			} else {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()

	now := unixMillis(normalizeTime(deletedAt))
	stmts := []struct {
		label string
		sql   string
		args  []any
	}{
		{"delete user note search", deleteUserNoteSearchSQL, []any{string(userID)}},
		{"queue user orphaned media", queueUserOrphanedMediaSQL, []any{now, string(userID)}},
		{"delete user image uploads", deleteUserImageUploadsSQL, []any{string(userID)}},
		{"delete user notes", deleteUserNotesSQL, []any{string(userID)}},
		{"delete user authors", deleteUserAuthorsSQL, []any{string(userID)}},
		{"delete user login identities", deleteUserLoginIdentitiesSQL, []any{string(userID)}},
		{"delete user sessions", deleteUserSessionsSQL, []any{string(userID)}},
		{"delete user", deleteUserSQL, []any{string(userID)}},
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			return fmt.Errorf("%s: %w", stmt.label, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user: %w", err)
	}
	return nil
}

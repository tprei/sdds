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
	// Upload rows outlive their owner: they are the ledger for the bytes in the
	// object store, and only the retention sweep deletes those bytes -- it
	// claims rows in the "deleting" and "expired" states. consumed_note_id and
	// the write lease are cleared so the row satisfies the table CHECK
	// constraint and no writer still holds it.
	detachUserImageUploadsSQL = `
		UPDATE image_uploads
		SET state = 'deleting', consumed_note_id = NULL, write_lease_until = NULL, updated_at = ?
		WHERE user_id = ? AND state IN ('pending', 'ready', 'consumed')`

	// note_search is an FTS5 virtual table and cannot carry a foreign key, so
	// its rows go by hand while the notes still resolve, as in DeleteNote.
	deleteUserNoteSearchSQL = `
		DELETE FROM note_search WHERE note_id IN (SELECT id FROM notes WHERE user_id = ?)`

	// Everything else cascades from this row: the author profile, login
	// identities, sessions, note-create requests, the notes and every row
	// hanging off them, the comments and reactions left on other people's
	// notes, filed reports, events, and contact channels with their tokens.
	deleteUserRowSQL = `DELETE FROM users WHERE id = ?`
)

// DeleteUser hard-deletes an account and everything personal to it in one
// transaction. Foreign-key cascades from the users row do the work; the two
// explicit statements cover what a foreign key cannot reach -- the FTS5 search
// rows, and the image_uploads ledger, handed to the retention sweep so its
// object bytes are reclaimed instead of orphaned.
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
	uid := string(userID)

	if _, err := tx.ExecContext(ctx, detachUserImageUploadsSQL, now, uid); err != nil {
		return fmt.Errorf("detach user image uploads: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteUserNoteSearchSQL, uid); err != nil {
		return fmt.Errorf("delete user note search: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteUserRowSQL, uid); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user: %w", err)
	}
	return nil
}

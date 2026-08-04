package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	findNoteOwnerSQL = `SELECT user_id FROM notes WHERE id = ?`
	updateNoteSQL    = `
		UPDATE notes
		SET title = ?, body = ?, category_slug = ?, updated_at = ?
		WHERE id = ?
	`
	deleteNoteSearchSQL = `DELETE FROM note_search WHERE note_id = ?`
	// Detaching a consumed upload from a deleted note moves it into the retryable
	// "deleting" state with no lease, which the upload retention sweep already
	// claims and uses to delete the stored object bytes. consumed_note_id is
	// cleared alongside the state change so the row satisfies the table CHECK
	// constraint (a non-"consumed" row must not reference a note).
	detachConsumedNoteImagesSQL = `
		UPDATE image_uploads
		SET state = 'deleting', consumed_note_id = NULL, write_lease_until = NULL, updated_at = ?
		WHERE consumed_note_id = ? AND state = 'consumed'`
	deleteNoteSQL = `DELETE FROM notes WHERE id = ?`
)

var _ note.EditStore = (*NoteStore)(nil)

// noteOwnerGuard reads the owner of a note inside the transaction and maps the
// result to the domain's not-found and forbidden sentinels.
func noteOwnerGuard(ctx context.Context, tx *sql.Tx, id string, userID user.UserID) error {
	var owner string
	err := tx.QueryRowContext(ctx, findNoteOwnerSQL, id).Scan(&owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return note.ErrNoteNotFound
		}
		return fmt.Errorf("read note owner: %w", err)
	}
	if owner != string(userID) {
		return note.ErrNoteForbidden
	}
	return nil
}

// UpdateNote rewrites a note's text and category and re-indexes its lexical and
// semantic search rows in a single transaction. Authorship is re-checked inside
// the transaction even though Editor already checked it, so the store method is
// safe to call directly.
func (store *NoteStore) UpdateNote(ctx context.Context, input note.UpdateInput) (updated note.Note, err error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return note.Note{}, fmt.Errorf("begin update note: %w", err)
	}
	now := normalizeTime(store.clock())
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			rollbackErr = fmt.Errorf("rollback update note: %w", rollbackErr)
			if err == nil {
				err = rollbackErr
			} else {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()

	if err := noteOwnerGuard(ctx, tx, input.NoteID, input.UserID); err != nil {
		return note.Note{}, err
	}
	if err := validateActiveCategory(ctx, tx, input.CategorySlug); err != nil {
		return note.Note{}, err
	}
	if _, err := tx.ExecContext(
		ctx,
		updateNoteSQL,
		input.Title,
		input.Body,
		string(input.CategorySlug),
		unixMillis(now),
		input.NoteID,
	); err != nil {
		return note.Note{}, fmt.Errorf("update note: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteNoteSearchSQL, input.NoteID); err != nil {
		return note.Note{}, fmt.Errorf("delete note search: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertNoteSearchSQL, input.NoteID, input.Title, input.Body); err != nil {
		return note.Note{}, fmt.Errorf("insert note search: %w", err)
	}
	if err := upsertNoteEmbedding(ctx, tx, input.NoteID, input.Embedding, now); err != nil {
		return note.Note{}, err
	}

	updated, err = loadNoteWithOrderedImages(ctx, tx, tx, input.NoteID, input.UserID)
	if err != nil {
		return note.Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return note.Note{}, fmt.Errorf("commit update note: %w", err)
	}
	return updated, nil
}

// DeleteNote hard-deletes a note. note_search is cleared explicitly because the
// FTS5 table has no foreign key; every other child table cascades through the
// notes row delete (PRAGMA foreign_keys = ON).
func (store *NoteStore) DeleteNote(ctx context.Context, id string, userID user.UserID) (err error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete note: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			rollbackErr = fmt.Errorf("rollback delete note: %w", rollbackErr)
			if err == nil {
				err = rollbackErr
			} else {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()

	if err := noteOwnerGuard(ctx, tx, id, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, deleteNoteSearchSQL, id); err != nil {
		return fmt.Errorf("delete note search: %w", err)
	}
	if _, err := tx.ExecContext(ctx, detachConsumedNoteImagesSQL, unixMillis(normalizeTime(store.clock())), id); err != nil {
		return fmt.Errorf("detach consumed note images: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteNoteSQL, id); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete note: %w", err)
	}
	return nil
}

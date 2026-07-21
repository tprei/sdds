package sqlite

import (
	"context"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	insertNoteUsefulReactionSQL = `
		INSERT OR IGNORE INTO note_useful_reactions (note_id, user_id, created_at)
		VALUES (?, ?, ?)
	`
	deleteNoteUsefulReactionSQL = `
		DELETE FROM note_useful_reactions
		WHERE note_id = ? AND user_id = ?
	`
)

var _ note.UsefulStore = (*NoteStore)(nil)

// MarkUseful records that a user marked the note useful. The composite primary
// key plus INSERT OR IGNORE make repeated marks idempotent: a duplicate mark
// returns nil without inserting a second row or inflating the useful count.
func (store *NoteStore) MarkUseful(ctx context.Context, input note.MarkUsefulInput) error {
	if _, err := store.db.ExecContext(
		ctx,
		insertNoteUsefulReactionSQL,
		input.NoteID,
		string(input.UserID),
		unixMillis(normalizeTime(store.clock())),
	); err != nil {
		return fmt.Errorf("mark note useful: %w", err)
	}
	return nil
}

// UnmarkUseful removes a user's useful mark. Deleting an absent row is success
// so callers can retry unmarks without distinguishing a missing reaction.
func (store *NoteStore) UnmarkUseful(ctx context.Context, input note.UnmarkUsefulInput) error {
	if _, err := store.db.ExecContext(
		ctx,
		deleteNoteUsefulReactionSQL,
		input.NoteID,
		string(input.UserID),
	); err != nil {
		return fmt.Errorf("unmark note useful: %w", err)
	}
	return nil
}

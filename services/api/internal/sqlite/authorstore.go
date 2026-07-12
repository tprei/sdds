package sqlite

import (
	"context"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	listAuthorNotesSQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE authors.id = ?
		ORDER BY notes.created_at DESC, notes.id DESC
		LIMIT ?
	`
	listAuthorNotesAfterSQL = `
		SELECT
			notes.id,
			notes.user_id,
			notes.title,
			notes.body,
			notes.category_slug,
			notes.place_slug,
			authors.id,
			authors.display_name,
			notes.created_at,
			notes.updated_at
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE authors.id = ?
			AND (
				notes.created_at < ?
				OR (notes.created_at = ? AND notes.id < ?)
			)
		ORDER BY notes.created_at DESC, notes.id DESC
		LIMIT ?
	`
)

var _ note.AuthorNoteStore = (*NoteStore)(nil)

func (store *NoteStore) ListAuthorNotes(ctx context.Context, input note.AuthorNotesInput) (page note.AuthorNotesPage, err error) {
	input = note.NormalizeAuthorNotesInput(input)
	if problems := note.ValidateAuthorNotesInput(input); len(problems) > 0 {
		return note.AuthorNotesPage{}, fmt.Errorf("list author notes: invalid input")
	}

	fetchLimit := input.Limit + 1
	query := listAuthorNotesSQL
	args := []any{input.AuthorID, fetchLimit}
	if input.After != nil {
		createdAt := unixMillis(input.After.CreatedAt)
		query = listAuthorNotesAfterSQL
		args = []any{input.AuthorID, createdAt, createdAt, input.After.ID, fetchLimit}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return note.AuthorNotesPage{}, fmt.Errorf("query author notes: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close author notes rows: %w", closeErr)
		}
	}()

	notes := make([]note.Note, 0, fetchLimit)
	for rows.Next() {
		found, err := scanNote(rows)
		if err != nil {
			return note.AuthorNotesPage{}, err
		}
		notes = append(notes, found)
	}
	if err := rows.Err(); err != nil {
		return note.AuthorNotesPage{}, fmt.Errorf("read author notes: %w", err)
	}

	page.Notes = notes
	if len(notes) > input.Limit {
		page.Notes = notes[:input.Limit]
		page.HasMore = true
	}
	return page, nil
}

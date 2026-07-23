package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	listAuthorNotesSQL = `
		SELECT
			notes.author_page_key,
		` + noteProjectionSQL + `
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE authors.id = ?
		ORDER BY notes.created_at DESC, notes.author_page_key DESC
		LIMIT ?
	`
	listAuthorNotesAfterSQL = `
		SELECT
			notes.author_page_key,
		` + noteProjectionSQL + `
		FROM notes
		JOIN authors ON authors.user_id = notes.user_id
		WHERE authors.id = ?
			AND (
				notes.created_at < ?
				OR (notes.created_at = ? AND notes.author_page_key < ?)
			)
		ORDER BY notes.created_at DESC, notes.author_page_key DESC
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
	args := []any{string(input.ViewerUserID), input.AuthorID, fetchLimit}
	if input.After != nil {
		createdAt := unixMillis(input.After.CreatedAt)
		query = listAuthorNotesAfterSQL
		args = []any{string(input.ViewerUserID), input.AuthorID, createdAt, createdAt, input.After.PageKey, fetchLimit}
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

	notes := make([]note.AuthorNote, 0, fetchLimit)
	for rows.Next() {
		found, err := scanAuthorNote(rows)
		if err != nil {
			return note.AuthorNotesPage{}, err
		}
		notes = append(notes, found)
	}
	if err := rows.Err(); err != nil {
		return note.AuthorNotesPage{}, fmt.Errorf("read author notes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return note.AuthorNotesPage{}, fmt.Errorf("close author notes rows: %w", err)
	}

	page.Notes = notes
	if len(notes) > input.Limit {
		page.Notes = notes[:input.Limit]
		page.HasMore = true
	}

	hydrated := make([]note.Note, len(page.Notes))
	for index := range page.Notes {
		hydrated[index] = page.Notes[index].Note
	}
	if err := store.loadNotesWithOrderedImages(ctx, hydrated); err != nil {
		return note.AuthorNotesPage{}, fmt.Errorf("load author note images: %w", err)
	}
	for index := range page.Notes {
		page.Notes[index].Note = hydrated[index]
	}
	return page, nil
}

func scanAuthorNote(rows *sql.Rows) (note.AuthorNote, error) {
	var pageKey int64
	found, err := scanNoteValues(func(dest ...any) error {
		destinations := append([]any{&pageKey}, dest...)
		return rows.Scan(destinations...)
	})
	if err != nil {
		return note.AuthorNote{}, err
	}
	return note.AuthorNote{
		Note: found,
		Position: note.AuthorNotePosition{
			CreatedAt: found.CreatedAt,
			PageKey:   pageKey,
		},
	}, nil
}

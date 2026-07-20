package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tprei/sdds/services/api/internal/note"
)

const listNoteImagesSQL = `
	SELECT
		id,
		note_id,
		content_type,
		byte_size,
		width,
		height,
		position,
		created_at,
		updated_at
	FROM note_images
	WHERE note_id IN (%s)
	ORDER BY note_id ASC, position ASC
`

type noteLookupQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type noteImageQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadNoteWithOrderedImages(ctx context.Context, noteQuery noteLookupQuerier, imageQuery noteImageQuerier, id string) (note.Note, error) {
	found, err := scanNoteRow(noteQuery.QueryRowContext(ctx, findNoteSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return note.Note{}, note.ErrNoteNotFound
		}
		return note.Note{}, err
	}
	notes := []note.Note{found}
	if err := loadOrderedImagesForNotes(ctx, imageQuery, notes); err != nil {
		return note.Note{}, err
	}
	return notes[0], nil
}

func (store *NoteStore) loadNotesWithOrderedImages(ctx context.Context, notes []note.Note) error {
	return loadOrderedImagesForNotes(ctx, store.db, notes)
}

func loadOrderedImagesForNotes(ctx context.Context, queryer noteImageQuerier, notes []note.Note) (err error) {
	if len(notes) == 0 {
		return nil
	}

	placeholders := make([]string, len(notes))
	args := make([]any, len(notes))
	indexes := make(map[string]int, len(notes))
	for index := range notes {
		placeholders[index] = "?"
		args[index] = notes[index].ID
		indexes[notes[index].ID] = index
		if notes[index].Images == nil {
			notes[index].Images = make([]note.Image, 0)
		}
	}

	rows, err := queryer.QueryContext(ctx, fmt.Sprintf(listNoteImagesSQL, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return fmt.Errorf("query note images: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close note image rows: %w", closeErr)
		}
	}()

	for rows.Next() {
		var image note.Image
		var noteID string
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(
			&image.ID,
			&noteID,
			&image.ContentType,
			&image.ByteSize,
			&image.Width,
			&image.Height,
			&image.Position,
			&createdAt,
			&updatedAt,
		); err != nil {
			return fmt.Errorf("scan note image: %w", err)
		}

		index, ok := indexes[noteID]
		if !ok {
			continue
		}
		image.CreatedAt = timeFromUnixMillis(createdAt)
		image.UpdatedAt = timeFromUnixMillis(updatedAt)
		notes[index].Images = append(notes[index].Images, image)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read note images: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close note image rows: %w", err)
	}
	return nil
}

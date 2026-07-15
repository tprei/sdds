package sqlite

import (
	"context"
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

func (store *NoteStore) hydrateNoteImages(ctx context.Context, notes []note.Note) error {
	return hydrateNoteImagesWithQueryer(ctx, store.db, notes)
}

func hydrateNoteImagesWithQueryer(ctx context.Context, queryer noteCreateQueryer, notes []note.Note) (err error) {
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

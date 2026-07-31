package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/author"
)

const findPublicAuthorSQL = `
	SELECT
		authors.id,
		authors.display_name,
		COUNT(notes.id),
		COALESCE((
			SELECT COUNT(*)
			FROM note_useful_reactions
			JOIN notes AS useful_notes ON useful_notes.id = note_useful_reactions.note_id
			WHERE useful_notes.user_id = authors.user_id
		), 0)
	FROM authors
	LEFT JOIN notes ON notes.user_id = authors.user_id
	WHERE authors.id = ?
	GROUP BY authors.id, authors.display_name
`

var _ author.PublicAuthorStore = (*UserStore)(nil)

func (store *UserStore) FindPublicAuthor(ctx context.Context, authorID author.AuthorID) (author.PublicAuthor, error) {
	var profile author.PublicAuthor
	var id string
	err := store.db.QueryRowContext(ctx, findPublicAuthorSQL, authorID).Scan(
		&id,
		&profile.DisplayName,
		&profile.NoteCount,
		&profile.UsefulReceivedCount,
	)
	if err == nil {
		profile.ID = author.AuthorID(id)
		return profile, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return author.PublicAuthor{}, author.ErrAuthorNotFound
	}
	return author.PublicAuthor{}, fmt.Errorf("find public author: %w", err)
}

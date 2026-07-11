package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/user"
)

const findPublicAuthorSQL = `
	SELECT authors.id, authors.display_name, COUNT(notes.id)
	FROM authors
	LEFT JOIN notes ON notes.user_id = authors.user_id
	WHERE authors.id = ?
	GROUP BY authors.id, authors.display_name
`

var _ user.PublicAuthorStore = (*UserStore)(nil)

func (store *UserStore) FindPublicAuthor(ctx context.Context, authorID user.AuthorID) (user.PublicAuthor, error) {
	var author user.PublicAuthor
	var id string
	err := store.db.QueryRowContext(ctx, findPublicAuthorSQL, authorID).Scan(&id, &author.DisplayName, &author.NoteCount)
	if err == nil {
		author.ID = user.AuthorID(id)
		return author, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return user.PublicAuthor{}, user.ErrAuthorNotFound
	}
	return user.PublicAuthor{}, fmt.Errorf("find public author: %w", err)
}

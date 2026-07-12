package author

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type AuthorID string

var ErrAuthorNotFound = errors.New("author not found")

type PublicAuthor struct {
	ID          AuthorID
	DisplayName string
	NoteCount   int64
}

type PublicAuthorStore interface {
	FindPublicAuthor(context.Context, AuthorID) (PublicAuthor, error)
}

func NewAuthorID() (AuthorID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return AuthorID(id.String()), nil
}

package user

import "context"

type PublicAuthor struct {
	ID          AuthorID
	DisplayName string
	NoteCount   int64
}

type PublicAuthorStore interface {
	FindPublicAuthor(context.Context, AuthorID) (PublicAuthor, error)
}

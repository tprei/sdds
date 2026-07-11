package note

import (
	"context"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	AuthorNotesDefaultLimit = 20
	AuthorNotesMaxLimit     = 50
)

type AuthorNotePosition struct {
	CreatedAt time.Time
	ID        string
}

type AuthorNotesInput struct {
	AuthorID user.AuthorID
	Limit    int
	After    *AuthorNotePosition
}

type AuthorNotesPage struct {
	Notes   []Note
	HasMore bool
}

type AuthorNoteStore interface {
	ListAuthorNotes(context.Context, AuthorNotesInput) (AuthorNotesPage, error)
}

func ValidateAuthorNotesInput(input AuthorNotesInput) []ValidationProblem {
	problems := make([]ValidationProblem, 0, 1)
	if input.Limit < 1 || input.Limit > AuthorNotesMaxLimit {
		return append(problems, ValidationProblem{Field: "limit", Message: "invalid"})
	}
	return problems
}

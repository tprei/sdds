package note

import (
	"context"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
)

const (
	AuthorNotesDefaultLimit = 20
	AuthorNotesMaxLimit     = 50
)

type AuthorNotePosition struct {
	CreatedAt time.Time
	RowID     int64
}

type AuthorNote struct {
	Note     Note
	Position AuthorNotePosition
}

type AuthorNotesInput struct {
	AuthorID author.AuthorID
	Limit    int
	After    *AuthorNotePosition
}

type AuthorNotesPage struct {
	Notes   []AuthorNote
	HasMore bool
}

type AuthorNoteStore interface {
	ListAuthorNotes(context.Context, AuthorNotesInput) (AuthorNotesPage, error)
}

func NormalizeAuthorNotesInput(input AuthorNotesInput) AuthorNotesInput {
	if input.Limit == 0 {
		input.Limit = AuthorNotesDefaultLimit
	}
	return input
}

func ValidateAuthorNotesInput(input AuthorNotesInput) []ValidationProblem {
	problems := make([]ValidationProblem, 0, 1)
	if input.Limit < 1 || input.Limit > AuthorNotesMaxLimit {
		return append(problems, ValidationProblem{Field: "limit", Message: "invalid"})
	}
	return problems
}

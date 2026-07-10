package note

import (
	"context"
	"errors"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

var ErrNoteNotFound = errors.New("note not found")

type Note struct {
	ID           string
	UserID       user.UserID
	Title        string
	Body         string
	CategorySlug CategorySlug
	PlaceSlug    PlaceSlug
	Author       AuthorSummary
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AuthorSummary struct {
	ID          user.AuthorID
	DisplayName string
}

type CreateInput struct {
	UserID       user.UserID
	Title        string
	Body         string
	CategorySlug CategorySlug
	PlaceSlug    PlaceSlug
}

type Store interface {
	CreateNote(ctx context.Context, input CreateInput) (Note, error)
	FindNote(ctx context.Context, id string) (Note, error)
	ListRecentNotes(ctx context.Context, input ListInput) ([]Note, error)
	SearchNotes(ctx context.Context, input SearchInput) ([]Note, error)
}

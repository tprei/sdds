package note

import (
	"context"
	"errors"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
)

var ErrNoteNotFound = errors.New("note not found")

type Image struct {
	ID          string
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Note struct {
	ID           string
	UserID       user.UserID
	Title        string
	Body         string
	CategorySlug CategorySlug
	PlaceSlug    PlaceSlug
	Author       AuthorSummary
	Images       []Image
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AuthorSummary struct {
	ID          author.AuthorID
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

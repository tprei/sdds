package note

import (
	"context"
	"errors"
	"time"
)

var ErrNoteNotFound = errors.New("note not found")

type Note struct {
	ID           string
	Title        string
	Body         string
	CategorySlug CategorySlug
	PlaceSlug    PlaceSlug
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateInput struct {
	Title        string
	Body         string
	CategorySlug CategorySlug
	PlaceSlug    PlaceSlug
}

type Store interface {
	CreateNote(ctx context.Context, input CreateInput) (Note, error)
	FindNote(ctx context.Context, id string) (Note, error)
	ListRecentNotes(ctx context.Context, limit int) ([]Note, error)
	SearchNotes(ctx context.Context, input SearchInput) ([]Note, error)
}

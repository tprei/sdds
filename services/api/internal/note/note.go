package note

import (
	"context"
	"time"
)

type Note struct {
	ID           string
	Title        string
	Body         string
	CategorySlug CategorySlug
	CitySlug     CitySlug
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateInput struct {
	Title        string
	Body         string
	CategorySlug CategorySlug
	CitySlug     CitySlug
}

type Store interface {
	CreateNote(ctx context.Context, input CreateInput) (Note, error)
	ListRecentNotes(ctx context.Context, limit int) ([]Note, error)
}

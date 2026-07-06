package note

import (
	"context"
	"errors"
)

var ErrCategoryNotFound = errors.New("category not found")
var ErrPlaceNotFound = errors.New("place not found")

type Catalog interface {
	ListCategories(ctx context.Context) ([]Category, error)
	ListPlaces(ctx context.Context) ([]Place, error)
	FindActiveCategory(ctx context.Context, slug CategorySlug) (Category, error)
	FindActivePlace(ctx context.Context, slug PlaceSlug) (Place, error)
}

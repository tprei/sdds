package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	listCategoriesSQL = `
		SELECT slug, label, active, display_order
		FROM categories
		ORDER BY display_order ASC, label ASC, slug ASC
	`
	listPlacesSQL = `
		SELECT slug, label, active, display_order
		FROM places
		ORDER BY display_order ASC, label ASC, slug ASC
	`
	findActiveCategorySQL = `
		SELECT slug, label, active, display_order
		FROM categories
		WHERE slug = ? AND active = 1
	`
	findActivePlaceSQL = `
		SELECT slug, label, active, display_order
		FROM places
		WHERE slug = ? AND active = 1
	`
)

var _ note.Catalog = (*CatalogStore)(nil)

type CatalogStore struct {
	db *sql.DB
}

func NewCatalogStore(db *sql.DB) *CatalogStore {
	return &CatalogStore{db: db}
}

func (store *CatalogStore) ListCategories(ctx context.Context) (categories []note.Category, err error) {
	rows, err := store.db.QueryContext(ctx, listCategoriesSQL)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close category rows: %w", closeErr)
		}
	}()

	categories = make([]note.Category, 0)
	for rows.Next() {
		category, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}

	return categories, nil
}

func (store *CatalogStore) ListPlaces(ctx context.Context) (places []note.Place, err error) {
	rows, err := store.db.QueryContext(ctx, listPlacesSQL)
	if err != nil {
		return nil, fmt.Errorf("query places: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close place rows: %w", closeErr)
		}
	}()

	places = make([]note.Place, 0)
	for rows.Next() {
		place, err := scanPlace(rows)
		if err != nil {
			return nil, err
		}
		places = append(places, place)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read places: %w", err)
	}

	return places, nil
}

func (store *CatalogStore) FindActiveCategory(ctx context.Context, slug note.CategorySlug) (note.Category, error) {
	category, err := scanCategoryRow(store.db.QueryRowContext(ctx, findActiveCategorySQL, string(slug)))
	if err == nil {
		return category, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return note.Category{}, note.ErrCategoryNotFound
	}
	return note.Category{}, fmt.Errorf("find active category: %w", err)
}

func (store *CatalogStore) FindActivePlace(ctx context.Context, slug note.PlaceSlug) (note.Place, error) {
	place, err := scanPlaceRow(store.db.QueryRowContext(ctx, findActivePlaceSQL, string(slug)))
	if err == nil {
		return place, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return note.Place{}, note.ErrPlaceNotFound
	}
	return note.Place{}, fmt.Errorf("find active place: %w", err)
}

func scanCategory(rows *sql.Rows) (note.Category, error) {
	return scanCategoryValues(rows.Scan)
}

func scanCategoryRow(row *sql.Row) (note.Category, error) {
	return scanCategoryValues(row.Scan)
}

func scanCategoryValues(scan func(dest ...any) error) (note.Category, error) {
	var category note.Category
	var slug string
	var active bool
	if err := scan(&slug, &category.Label, &active, &category.DisplayOrder); err != nil {
		return note.Category{}, fmt.Errorf("scan category: %w", err)
	}

	category.Slug = note.CategorySlug(slug)
	category.Active = active
	return category, nil
}

func scanPlace(rows *sql.Rows) (note.Place, error) {
	return scanPlaceValues(rows.Scan)
}

func scanPlaceRow(row *sql.Row) (note.Place, error) {
	return scanPlaceValues(row.Scan)
}

func scanPlaceValues(scan func(dest ...any) error) (note.Place, error) {
	var place note.Place
	var slug string
	var active bool
	if err := scan(&slug, &place.Label, &active, &place.DisplayOrder); err != nil {
		return note.Place{}, fmt.Errorf("scan place: %w", err)
	}

	place.Slug = note.PlaceSlug(slug)
	place.Active = active
	return place, nil
}

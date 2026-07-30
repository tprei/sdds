package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestCatalogStoreListsCategories(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	found, err := NewCatalogStore(db).ListCategories(ctx)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}

	if diff := cmp.Diff(note.Categories, found); diff != "" {
		t.Fatalf("categories mismatch (-want +got):\n%s", diff)
	}
}

func TestCatalogStoreFindsActiveCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	found, err := NewCatalogStore(db).FindActiveCategory(ctx, note.CategorySlugFood)
	if err != nil {
		t.Fatalf("find active category: %v", err)
	}

	if diff := cmp.Diff(note.Categories[1], found); diff != "" {
		t.Fatalf("category mismatch (-want +got):\n%s", diff)
	}
}

func TestCatalogStoreTreatsUnknownCategoryAsNotFound(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	_, err := NewCatalogStore(db).FindActiveCategory(ctx, "missing")
	if !errors.Is(err, note.ErrCategoryNotFound) {
		t.Fatalf("find active category error = %v, want ErrCategoryNotFound", err)
	}
}

func TestCatalogStoreTreatsInactiveCategoryAsNotFound(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	if _, err := db.ExecContext(ctx, `UPDATE categories SET active = 0 WHERE slug = ?`, note.CategorySlugFood); err != nil {
		t.Fatalf("deactivate category: %v", err)
	}

	_, err := NewCatalogStore(db).FindActiveCategory(ctx, note.CategorySlugFood)
	if !errors.Is(err, note.ErrCategoryNotFound) {
		t.Fatalf("find active category error = %v, want ErrCategoryNotFound", err)
	}
}

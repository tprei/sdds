package sqlite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestApplyMigrationsCreatesInitialSchema(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	tables := []string{"schema_migrations", "categories", "cities", "notes"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
				t.Fatalf("query table %s: %v", table, err)
			}
			if count != 1 {
				t.Fatalf("table %s count = %d, want 1", table, count)
			}
		})
	}
}

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations again: %v", err)
	}
}

func TestApplyMigrationsSeedsControlledMetadata(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	wantCategories := make(map[string]string, len(note.Categories))
	gotCategories := make(map[string]string, len(note.Categories))
	for _, category := range note.Categories {
		wantCategories[string(category.Slug)] = category.Label
		var label string
		if err := db.QueryRowContext(ctx, `SELECT label FROM categories WHERE slug = ?`, category.Slug).Scan(&label); err != nil {
			t.Fatalf("query category %s: %v", category.Slug, err)
		}
		gotCategories[string(category.Slug)] = label
	}
	if diff := cmp.Diff(wantCategories, gotCategories); diff != "" {
		t.Fatalf("categories mismatch (-want +got):\n%s", diff)
	}

	wantCities := make(map[string]string, len(note.Cities))
	gotCities := make(map[string]string, len(note.Cities))
	for _, city := range note.Cities {
		wantCities[string(city.Slug)] = city.Label
		var label string
		if err := db.QueryRowContext(ctx, `SELECT label FROM cities WHERE slug = ?`, city.Slug).Scan(&label); err != nil {
			t.Fatalf("query city %s: %v", city.Slug, err)
		}
		gotCities[string(city.Slug)] = label
	}
	if diff := cmp.Diff(wantCities, gotCities); diff != "" {
		t.Fatalf("cities mismatch (-want +got):\n%s", diff)
	}
}

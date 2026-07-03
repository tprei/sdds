package sqlite

import (
	"context"
	"testing"

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

	for _, category := range note.Categories {
		var label string
		if err := db.QueryRowContext(ctx, `SELECT label FROM categories WHERE slug = ?`, category.Slug).Scan(&label); err != nil {
			t.Fatalf("query category %s: %v", category.Slug, err)
		}
		if label != category.Label {
			t.Fatalf("category %s label = %s, want %s", category.Slug, label, category.Label)
		}
	}

	for _, city := range note.Cities {
		var label string
		if err := db.QueryRowContext(ctx, `SELECT label FROM cities WHERE slug = ?`, city.Slug).Scan(&label); err != nil {
			t.Fatalf("query city %s: %v", city.Slug, err)
		}
		if label != city.Label {
			t.Fatalf("city %s label = %s, want %s", city.Slug, label, city.Label)
		}
	}
}

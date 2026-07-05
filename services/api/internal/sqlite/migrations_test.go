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

	tables := []string{"schema_migrations", "categories", "cities", "notes", "note_search"}
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

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations again: %v", err)
	}
}

func TestApplyMigrationsSeedsControlledMetadata(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

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

func TestApplyMigrationsIndexesExistingNotes(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	if _, err := db.ExecContext(ctx, createSchemaMigrationsSQL); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	initialMigration, err := migrations.ReadFile("migrations/000001_initial_notes.sql")
	if err != nil {
		t.Fatalf("read initial migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(initialMigration)); err != nil {
		t.Fatalf("apply initial migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, recordMigrationSQL, "000001_initial_notes"); err != nil {
		t.Fatalf("record initial migration: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`
			INSERT INTO notes (id, title, body, category_slug, city_slug, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
		"existing-note",
		"Café bom",
		"Tem pão de queijo decente.",
		note.CategorySlugComida,
		note.CitySlugSaoPaulo,
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert existing note: %v", err)
	}

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
	}

	found, err := NewNoteStore(db).SearchNotes(ctx, note.SearchInput{
		Query: "cafe",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	gotIDs := make([]string, 0, len(found))
	for _, existing := range found {
		gotIDs = append(gotIDs, existing.ID)
	}
	wantIDs := []string{"existing-note"}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

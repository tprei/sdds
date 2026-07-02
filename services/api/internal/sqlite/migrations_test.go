package sqlite

import (
	"context"
	"testing"
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

	var categoryCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&categoryCount); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if categoryCount != 4 {
		t.Fatalf("category count = %d, want 4", categoryCount)
	}

	var cityCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cities`).Scan(&cityCount); err != nil {
		t.Fatalf("count cities: %v", err)
	}
	if cityCount != 3 {
		t.Fatalf("city count = %d, want 3", cityCount)
	}
}

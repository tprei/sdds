// Package sqlite test fixtures consolidate the production-faithful SQLite setup
// shared across repository behavior tests.
//
// Repository behavior tests MUST use openMigratedDatabase, which opens an
// in-memory database through the production Open path (foreign keys, busy
// timeout, one-connection constraint) and applies the full migration history.
// Migration-history tests MUST NOT reuse this helper; they build a deliberately
// partial schema via applyMigrationHistorySchema so the two setups cannot be
// confused.
package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

func openMigratedDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

// execer lets seed helpers target either a *sql.DB or a *sql.Tx so a test can
// seed inside a transaction it later rolls back.
type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertAuthorStoreUser(t *testing.T, ctx context.Context, db execer, userID user.UserID, authorID author.AuthorID, displayName string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, userID); err != nil {
		t.Fatalf("insert user %s: %v", userID, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO authors (id, user_id, display_name, created_at, updated_at) VALUES (?, ?, ?, 0, 0)`, authorID, userID, displayName); err != nil {
		t.Fatalf("insert author %s: %v", authorID, err)
	}
}

func insertAuthorStoreNote(t *testing.T, ctx context.Context, db execer, id string, userID user.UserID, createdAt int64) {
	t.Helper()
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO notes (id, user_id, title, body, category_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		userID,
		"Café bom",
		"Tem pão de queijo decente.",
		note.CategorySlugFood,
		createdAt,
		createdAt,
	); err != nil {
		t.Fatalf("insert note %s: %v", id, err)
	}
}

// insertBareUsefulStoreUser seeds a user row without an author, for tests that
// exercise useful-state read paths against an unauthenticated viewer identity.
func insertBareUsefulStoreUser(t *testing.T, ctx context.Context, db execer, userID user.UserID) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, userID); err != nil {
		t.Fatalf("insert bare user %s: %v", userID, err)
	}
}

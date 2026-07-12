package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestApplyMigrationsCreatesInitialSchema(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	tables := []string{
		"schema_migrations",
		"categories",
		"places",
		"notes",
		"note_search",
		"users",
		"authors",
		"user_login_identities",
		"sessions",
	}
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

func TestApplyMigrationsCreatesCatalogIndexes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	indexes := []string{
		"notes_recent_idx",
		"notes_category_idx",
		"notes_place_idx",
		"notes_user_idx",
		"notes_author_page_idx",
		"user_login_identities_user_idx",
		"user_login_identities_one_password_provider_per_user_idx",
		"sessions_user_idx",
		"sessions_active_expiry_idx",
	}
	for _, index := range indexes {
		t.Run(index, func(t *testing.T) {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
				t.Fatalf("query index %s: %v", index, err)
			}
			if count != 1 {
				t.Fatalf("index %s count = %d, want 1", index, count)
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

func TestApplyMigrationsSeedsCatalogs(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	wantCategories := make(map[string]note.Category, len(note.Categories))
	gotCategories := make(map[string]note.Category, len(note.Categories))
	for _, category := range note.Categories {
		wantCategories[string(category.Slug)] = category
		var got note.Category
		var slug string
		if err := db.QueryRowContext(ctx, `SELECT slug, label, active, display_order FROM categories WHERE slug = ?`, category.Slug).Scan(&slug, &got.Label, &got.Active, &got.DisplayOrder); err != nil {
			t.Fatalf("query category %s: %v", category.Slug, err)
		}
		got.Slug = note.CategorySlug(slug)
		gotCategories[string(category.Slug)] = got
	}
	if diff := cmp.Diff(wantCategories, gotCategories); diff != "" {
		t.Fatalf("categories mismatch (-want +got):\n%s", diff)
	}

	wantPlaces := make(map[string]note.Place, len(note.Places))
	gotPlaces := make(map[string]note.Place, len(note.Places))
	for _, place := range note.Places {
		wantPlaces[string(place.Slug)] = place
		var got note.Place
		var slug string
		if err := db.QueryRowContext(ctx, `SELECT slug, label, active, display_order FROM places WHERE slug = ?`, place.Slug).Scan(&slug, &got.Label, &got.Active, &got.DisplayOrder); err != nil {
			t.Fatalf("query place %s: %v", place.Slug, err)
		}
		got.Slug = note.PlaceSlug(slug)
		gotPlaces[string(place.Slug)] = got
	}
	if diff := cmp.Diff(wantPlaces, gotPlaces); diff != "" {
		t.Fatalf("places mismatch (-want +got):\n%s", diff)
	}
}

func TestLoginIdentityMigrationEnforcesSecretHashInvariants(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"user-id",
		"active",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err := db.ExecContext(
		ctx,
		`
			INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
		"password-without-secret",
		"user-id",
		"password",
		"local",
		"thiago",
		nil,
		int64(1782993600000),
		int64(1782993600000),
	)
	if err == nil {
		t.Fatal("insert password identity without secret_hash error = nil, want constraint error")
	}

	if _, err := db.ExecContext(
		ctx,
		`
			INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
		"oidc-without-secret",
		"user-id",
		"oidc",
		"google",
		"google-subject-id",
		nil,
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert oidc identity without secret_hash: %v", err)
	}

	_, err = db.ExecContext(
		ctx,
		`
			INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
		"oidc-with-secret",
		"user-id",
		"oidc",
		"apple",
		"apple-subject-id",
		"fake-secret",
		int64(1782993600000),
		int64(1782993600000),
	)
	if err == nil {
		t.Fatal("insert oidc identity with secret_hash error = nil, want constraint error")
	}
}

func TestLoginIdentityMigrationAllowsOnlyOnePasswordProviderPerUser(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"user-id",
		"active",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	insertPasswordIdentity := func(id string, normalizedIdentifier string) error {
		_, err := db.ExecContext(
			ctx,
			`
				INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`,
			id,
			"user-id",
			"password",
			"local",
			normalizedIdentifier,
			"secret-hash",
			int64(1782993600000),
			int64(1782993600000),
		)
		return err
	}

	if err := insertPasswordIdentity("first-password", "thiago"); err != nil {
		t.Fatalf("insert first password identity: %v", err)
	}
	if err := insertPasswordIdentity("second-password", "thiago-alt"); err == nil {
		t.Fatal("insert second password identity error = nil, want constraint error")
	}
}

func TestCatalogMigrationRequiresPlacesToReferenceCities(t *testing.T) {
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
	applyMigrationFiles(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs")

	_, err = db.ExecContext(
		ctx,
		`
			INSERT INTO places (slug, label, active, display_order)
			VALUES (?, ?, ?, ?)
		`,
		"curitiba",
		"Curitiba",
		true,
		40,
	)
	if err == nil {
		t.Fatal("insert place error = nil, want foreign key error")
	}
}

func TestNotePlaceMigrationPreservesExistingNotes(t *testing.T) {
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
	applyMigrationFiles(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs")

	if _, err := db.ExecContext(ctx, `UPDATE categories SET label = ?, active = 0, display_order = 99 WHERE slug = ?`, "Comida guardada", "comida"); err != nil {
		t.Fatalf("update legacy category: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE places SET label = ?, active = 0, display_order = 88 WHERE slug = ?`, "São Paulo guardado", "sao-paulo"); err != nil {
		t.Fatalf("update legacy place: %v", err)
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
		"comida",
		"sao-paulo",
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
	if len(found) != 1 {
		t.Fatalf("search note count = %d, want 1", len(found))
	}
	gotNote := found[0]
	if gotNote.ID != "existing-note" {
		t.Fatalf("search note id = %q, want existing-note", gotNote.ID)
	}
	if gotNote.CategorySlug != note.CategorySlugFood {
		t.Fatalf("search note category = %q, want %q", gotNote.CategorySlug, note.CategorySlugFood)
	}
	if gotNote.PlaceSlug != note.PlaceSlugSaoPaulo {
		t.Fatalf("search note place = %q, want %q", gotNote.PlaceSlug, note.PlaceSlugSaoPaulo)
	}

	var category note.Category
	var categorySlug string
	if err := db.QueryRowContext(ctx, `SELECT slug, label, active, display_order FROM categories WHERE slug = ?`, note.CategorySlugFood).Scan(&categorySlug, &category.Label, &category.Active, &category.DisplayOrder); err != nil {
		t.Fatalf("query migrated category: %v", err)
	}
	category.Slug = note.CategorySlug(categorySlug)
	wantCategory := note.Category{
		Slug:         note.CategorySlugFood,
		Label:        "Comida guardada",
		Active:       false,
		DisplayOrder: 99,
	}
	if diff := cmp.Diff(wantCategory, category); diff != "" {
		t.Fatalf("category mismatch (-want +got):\n%s", diff)
	}

	var place note.Place
	var placeSlug string
	if err := db.QueryRowContext(ctx, `SELECT slug, label, active, display_order FROM places WHERE slug = ?`, note.PlaceSlugSaoPaulo).Scan(&placeSlug, &place.Label, &place.Active, &place.DisplayOrder); err != nil {
		t.Fatalf("query migrated place: %v", err)
	}
	place.Slug = note.PlaceSlug(placeSlug)
	wantPlace := note.Place{
		Slug:         note.PlaceSlugSaoPaulo,
		Label:        "São Paulo guardado",
		Active:       false,
		DisplayOrder: 88,
	}
	if diff := cmp.Diff(wantPlace, place); diff != "" {
		t.Fatalf("place mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteOwnershipMigrationPreservesExistingNotes(t *testing.T) {
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
	applyMigrationFiles(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs", "000004_note_places", "000005_users_authors_sessions")

	if _, err := db.ExecContext(
		ctx,
		`
			INSERT INTO notes (id, title, body, category_slug, place_slug, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
		"existing-owned-note",
		"Cafe com pao",
		"Padaria boa perto do metrô.",
		"food",
		"sao-paulo",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert existing note: %v", err)
	}

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
	}

	var userIDType string
	var userIDNotNull int
	if err := db.QueryRowContext(ctx, `SELECT type, "notnull" FROM pragma_table_info('notes') WHERE name = 'user_id'`).Scan(&userIDType, &userIDNotNull); err != nil {
		t.Fatalf("query user_id column: %v", err)
	}
	if userIDType != "TEXT" {
		t.Fatalf("user_id type = %q, want TEXT", userIDType)
	}
	if userIDNotNull != 1 {
		t.Fatalf("user_id notnull = %d, want 1", userIDNotNull)
	}

	var ownerForeignKeys int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_list('notes') WHERE "from" = 'user_id' AND "table" = 'users' AND "to" = 'id'`).Scan(&ownerForeignKeys); err != nil {
		t.Fatalf("query user_id foreign key: %v", err)
	}
	if ownerForeignKeys != 1 {
		t.Fatalf("user_id foreign key count = %d, want 1", ownerForeignKeys)
	}

	var userState string
	var userCreatedAt int64
	var userUpdatedAt int64
	if err := db.QueryRowContext(ctx, `SELECT state, created_at, updated_at FROM users WHERE id = ?`, systemNoteOwnerUserID).Scan(&userState, &userCreatedAt, &userUpdatedAt); err != nil {
		t.Fatalf("query system user: %v", err)
	}
	if userState != "active" {
		t.Fatalf("system user state = %q, want active", userState)
	}
	if userCreatedAt != 0 || userUpdatedAt != 0 {
		t.Fatalf("system user timestamps = %d/%d, want 0/0", userCreatedAt, userUpdatedAt)
	}

	var authorUserID string
	var authorDisplayName string
	if err := db.QueryRowContext(ctx, `SELECT user_id, display_name FROM authors WHERE id = ?`, systemNoteOwnerAuthorID).Scan(&authorUserID, &authorDisplayName); err != nil {
		t.Fatalf("query system author: %v", err)
	}
	if authorUserID != string(systemNoteOwnerUserID) {
		t.Fatalf("system author user id = %q, want %q", authorUserID, systemNoteOwnerUserID)
	}
	if authorDisplayName != "sdds" {
		t.Fatalf("system author display name = %q, want sdds", authorDisplayName)
	}

	var migratedUserID string
	if err := db.QueryRowContext(ctx, `SELECT user_id FROM notes WHERE id = ?`, "existing-owned-note").Scan(&migratedUserID); err != nil {
		t.Fatalf("query migrated note user id: %v", err)
	}
	if migratedUserID != string(systemNoteOwnerUserID) {
		t.Fatalf("migrated note user id = %q, want %q", migratedUserID, systemNoteOwnerUserID)
	}

	for _, index := range []string{"notes_recent_idx", "notes_category_idx", "notes_place_idx", "notes_user_idx", "notes_author_page_idx"} {
		t.Run(index, func(t *testing.T) {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
				t.Fatalf("query index %s: %v", index, err)
			}
			if count != 1 {
				t.Fatalf("index %s count = %d, want 1", index, count)
			}
		})
	}

	found, err := NewNoteStore(db).SearchNotes(ctx, note.SearchInput{
		Query: "pao",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("search note count = %d, want 1", len(found))
	}
	gotNote := found[0]
	if gotNote.ID != "existing-owned-note" {
		t.Fatalf("search note id = %q, want existing-owned-note", gotNote.ID)
	}
	if gotNote.UserID != systemNoteOwnerUserID {
		t.Fatalf("search note user id = %q, want %q", gotNote.UserID, systemNoteOwnerUserID)
	}
	wantAuthor := note.AuthorSummary{ID: systemNoteOwnerAuthorID, DisplayName: "sdds"}
	if diff := cmp.Diff(wantAuthor, gotNote.Author); diff != "" {
		t.Fatalf("search note author mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteCursorMigrationPreservesLegacyNoteIDsAndTimestamps(t *testing.T) {
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
	applyMigrationFiles(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs", "000004_note_places", "000005_users_authors_sessions", "000006_note_ownership", "000007_author_notes_index")
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`,
		string(systemNoteOwnerUserID),
		int64(0),
		int64(0),
	); err != nil {
		t.Fatalf("insert legacy owner user: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO authors (id, user_id, display_name, created_at, updated_at) VALUES (?, ?, 'sdds', ?, ?)`,
		string(systemNoteOwnerAuthorID),
		string(systemNoteOwnerUserID),
		int64(0),
		int64(0),
	); err != nil {
		t.Fatalf("insert legacy owner author: %v", err)
	}

	legacyNotes := []struct {
		inputID   any
		storedID  string
		createdAt int64
		updatedAt int64
	}{
		{inputID: "", storedID: "", createdAt: 0, updatedAt: 0},
		{inputID: "legacy/id", storedID: "legacy/id", createdAt: -1, updatedAt: -2},
		{inputID: "emoji-😀", storedID: "emoji-😀", createdAt: 1782993600000, updatedAt: 1782993600000},
		{inputID: strings.Repeat("x", 260), storedID: strings.Repeat("x", 260), createdAt: 1782993600001, updatedAt: 1782993600001},
		{inputID: "nul-\x00-id", storedID: "nul-\x00-id", createdAt: 1782993600002, updatedAt: 1782993600002},
		{inputID: "same-id", storedID: "same-id", createdAt: 1782993600003, updatedAt: 1782993600003},
		{inputID: []byte("same-id"), storedID: "legacy-blob-id-7-73616d652d6964", createdAt: 1782993600004, updatedAt: 1782993600004},
		{inputID: "legacy-blob-id-9-626c6f622d6e6f74652d6964", storedID: "legacy-blob-id-9-626c6f622d6e6f74652d6964", createdAt: 1782993600005, updatedAt: 1782993600005},
		{inputID: []byte("blob-note-id"), storedID: "legacy-blob-id-9-626c6f622d6e6f74652d6964-1", createdAt: 1782993600006, updatedAt: 1782993600006},
	}
	for _, legacy := range legacyNotes {
		if _, err := db.ExecContext(
			ctx,
			`
				INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`,
			legacy.inputID,
			string(systemNoteOwnerUserID),
			"Cafe legado",
			"Nota legada com identificador antigo.",
			"food",
			"sao-paulo",
			legacy.createdAt,
			legacy.updatedAt,
		); err != nil {
			t.Fatalf("insert legacy note %q: %v", legacy.storedID, err)
		}
	}

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
	}

	for _, legacy := range legacyNotes {
		var storedID string
		var createdAt int64
		var updatedAt int64
		if err := db.QueryRowContext(ctx, `SELECT id, created_at, updated_at FROM notes WHERE id = ?`, legacy.storedID).Scan(&storedID, &createdAt, &updatedAt); err != nil {
			t.Fatalf("query migrated note %q: %v", legacy.storedID, err)
		}
		if storedID != legacy.storedID {
			t.Fatalf("migrated note id = %q, want %q", storedID, legacy.storedID)
		}
		if createdAt != legacy.createdAt {
			t.Fatalf("migrated note created_at = %d, want %d", createdAt, legacy.createdAt)
		}
		if updatedAt != legacy.updatedAt {
			t.Fatalf("migrated note updated_at = %d, want %d", updatedAt, legacy.updatedAt)
		}
		var searchRows int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_search WHERE note_id = ?`, legacy.storedID).Scan(&searchRows); err != nil {
			t.Fatalf("query search row %q: %v", legacy.storedID, err)
		}
		if searchRows != 1 {
			t.Fatalf("search rows for %q = %d, want 1", legacy.storedID, searchRows)
		}
	}
}

func TestNoteCursorMigrationEnforcesStoredCursorTypes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`,
		"cursor-user",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO authors (id, user_id, display_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"cursor-author",
		"cursor-user",
		"Cursor author",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert author: %v", err)
	}

	insertNote := func(id any, createdAt any, updatedAt any) error {
		_, err := db.ExecContext(
			ctx,
			`
				INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`,
			id,
			"cursor-user",
			"Cursor note",
			"Persisted cursor bounds.",
			"food",
			"sao-paulo",
			createdAt,
			updatedAt,
		)
		return err
	}

	for _, id := range []string{
		strings.Repeat("x", 240),
		strings.Repeat("y", 241),
		"unsafe-id&",
		strings.Repeat("😀", 100),
		"nul-\x00-id",
	} {
		if err := insertNote(id, 1782993600000, 1782993600000); err != nil {
			t.Fatalf("insert text note ID %q: %v", id, err)
		}
	}
	if err := insertNote("zero-created-at", 0, 1782993600000); err != nil {
		t.Fatalf("insert zero created_at: %v", err)
	}
	if err := insertNote("negative-created-at", -1, 1782993600000); err != nil {
		t.Fatalf("insert negative created_at: %v", err)
	}
	if err := insertNote("zero-updated-at", 1782993600000, 0); err != nil {
		t.Fatalf("insert zero updated_at: %v", err)
	}
	if err := insertNote("negative-updated-at", 1782993600000, -1); err != nil {
		t.Fatalf("insert negative updated_at: %v", err)
	}
	if err := insertNote([]byte("blob-note-id"), 1782993600000, 1782993600000); err == nil {
		t.Fatal("insert BLOB note ID error = nil, want constraint error")
	}
	if err := insertNote("text-created-at", "not-a-timestamp", int64(1782993600000)); err == nil {
		t.Fatal("insert text created_at error = nil, want constraint error")
	}
	if err := insertNote("real-created-at", 1e100, int64(1782993600000)); err == nil {
		t.Fatal("insert real created_at error = nil, want constraint error")
	}
}

func TestApplyMigrationsCreatesEmptySearchIndex(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	found, err := NewNoteStore(db).SearchNotes(ctx, note.SearchInput{Query: "cafe", Limit: 10})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("search note count = %d, want 0", len(found))
	}
}

func applyMigrationFiles(t *testing.T, ctx context.Context, db *sql.DB, versions ...string) {
	t.Helper()

	if _, err := db.ExecContext(ctx, createSchemaMigrationsSQL); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	for _, version := range versions {
		contents, err := migrations.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatalf("read migration %s: %v", version, err)
		}
		if _, err := db.ExecContext(ctx, string(contents)); err != nil {
			t.Fatalf("apply migration %s: %v", version, err)
		}
		if _, err := db.ExecContext(ctx, recordMigrationSQL, version); err != nil {
			t.Fatalf("record migration %s: %v", version, err)
		}
	}
}

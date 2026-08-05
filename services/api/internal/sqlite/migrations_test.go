package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestApplyMigrationsCreatesInitialSchema(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	tables := []string{
		"schema_migrations",
		"categories",
		"notes",
		"note_search",
		"users",
		"authors",
		"user_login_identities",
		"sessions",
		"note_images",
		"note_useful_reactions",
		"note_comments",
		"note_create_requests",
		"user_contact_channels",
		"user_contact_channel_tokens",
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
		"notes_user_idx",
		"notes_author_page_idx",
		"user_login_identities_user_idx",
		"user_login_identities_one_password_provider_per_user_idx",
		"sessions_user_idx",
		"sessions_active_expiry_idx",
		"note_comments_note_page_idx",
		"note_comments_user_idx",
		"note_useful_reactions_user_idx",
		"user_contact_channels_user_idx",
		"user_contact_channels_verified_value_idx",
		"user_contact_channel_tokens_channel_idx",
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
	applyMigrationHistorySchema(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs")

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

func TestCatalogMigrationPreservesModifiedCategoryAttributes(t *testing.T) {
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
	applyMigrationHistorySchema(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs")

	if _, err := db.ExecContext(ctx, `UPDATE categories SET label = ?, active = 0, display_order = 99 WHERE slug = ?`, "Comida guardada", "comida"); err != nil {
		t.Fatalf("update legacy category: %v", err)
	}

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
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
}

func TestDropPlacesMigrationRemovesPlaceDomainAndPreservesNotes(t *testing.T) {
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
	applyMigrationHistorySchema(t, ctx, db,
		"000001_initial_notes", "000002_note_search", "000003_catalogs",
		"000004_note_places", "000005_users_authors_sessions", "000006_note_ownership",
		"000007_author_notes_index", "000008_note_cursor_invariants", "000009_note_images",
		"000010_image_uploads", "000011_note_create_requests", "000012_note_useful",
		"000013_note_comments", "000014_reports", "000015_events",
	)

	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`, "drop-places-user", int64(0), int64(0)); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`
			INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
		"note-with-place",
		"drop-places-user",
		"Café bom",
		"Tem pão de queijo decente.",
		"food",
		"sao-paulo",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert note with place: %v", err)
	}

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply drop-places migration: %v", err)
	}

	var placesTableCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'places'`).Scan(&placesTableCount); err != nil {
		t.Fatalf("query places table: %v", err)
	}
	if placesTableCount != 0 {
		t.Fatalf("places table count = %d, want 0", placesTableCount)
	}

	var placeIndexCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'notes_place_idx'`).Scan(&placeIndexCount); err != nil {
		t.Fatalf("query notes_place_idx: %v", err)
	}
	if placeIndexCount != 0 {
		t.Fatalf("notes_place_idx count = %d, want 0", placeIndexCount)
	}

	var placeColumnCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('notes') WHERE name = 'place_slug'`).Scan(&placeColumnCount); err != nil {
		t.Fatalf("query notes.place_slug column: %v", err)
	}
	if placeColumnCount != 0 {
		t.Fatalf("notes.place_slug column count = %d, want 0", placeColumnCount)
	}

	var title, body, categorySlugValue string
	if err := db.QueryRowContext(ctx, `SELECT title, body, category_slug FROM notes WHERE id = ?`, "note-with-place").Scan(&title, &body, &categorySlugValue); err != nil {
		t.Fatalf("query migrated note: %v", err)
	}
	if title != "Café bom" || body != "Tem pão de queijo decente." || categorySlugValue != "food" {
		t.Fatalf("migrated note = (%q, %q, %q), want (Café bom, Tem pão de queijo decente., food)", title, body, categorySlugValue)
	}

	var noteCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notes`).Scan(&noteCount); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if noteCount != 1 {
		t.Fatalf("note count = %d, want 1", noteCount)
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
	applyMigrationHistorySchema(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs", "000004_note_places", "000005_users_authors_sessions")

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

	for _, index := range []string{"notes_recent_idx", "notes_category_idx", "notes_user_idx", "notes_author_page_idx"} {
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
	applyMigrationHistorySchema(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs", "000004_note_places", "000005_users_authors_sessions", "000006_note_ownership", "000007_author_notes_index")
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
				INSERT INTO notes (id, user_id, title, body, category_slug, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`,
			id,
			"cursor-user",
			"Cursor note",
			"Persisted cursor bounds.",
			"food",
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

func TestNoteEmbeddingsMigrationCascadesOnNoteDelete(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	// testCreateInput attaches a deterministic embedding, so CreateNote
	// already inserts the note_embeddings row atomically; this test only
	// needs to confirm the cascade removes it on delete.
	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
		Title:        "Wi-Fi estável",
		Body:         "várias tomadas e ninguém reclamou",
		CategorySlug: note.CategorySlug("food"),
	}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	var before int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_embeddings WHERE note_id = ?`, created.ID).Scan(&before); err != nil {
		t.Fatalf("count embeddings before delete: %v", err)
	}
	if before != 1 {
		t.Fatalf("embedding rows after create = %d, want 1", before)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, created.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_embeddings WHERE note_id = ?`, created.ID).Scan(&remaining); err != nil {
		t.Fatalf("count remaining embeddings: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("embedding rows after note delete = %d, want 0 (cascade)", remaining)
	}
}

func TestNoteEmbeddingsMigrationRejectsMismatchedVectorLength(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	// Seed the system user directly (bypassing CreateNote/the publisher path)
	// so this note has no embedding row yet, keeping the CHECK-constraint
	// assertion isolated from the atomic-publish insert.
	newTestNoteStore(db, time.Now)
	const noteID = "mismatched-vector-note"
	insertAuthorStoreNote(t, ctx, db, noteID, systemNoteOwnerUserID, 0)

	// A vector whose byte length is half of dimension*4 must be rejected by the
	// length(vector) = dimension * 4 CHECK constraint.
	shortBlob := encodeVector(make([]float32, note.EmbeddingDimension/2))
	_, err := db.ExecContext(ctx, `
		INSERT INTO note_embeddings (note_id, model_id, model_revision, dimension, source_sha256, vector, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, noteID, note.EmbeddingModelID, note.EmbeddingModelRevision, note.EmbeddingDimension,
		note.EmbeddingFingerprint("stale"), shortBlob, 0, 0)
	if err == nil {
		t.Fatal("expected CHECK constraint violation for mismatched vector length, got nil")
	}
}

func TestNoteImagesMigrationPreservesExistingTextOnlyNotes(t *testing.T) {
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
	applyMigrationHistorySchema(t, ctx, db, "000001_initial_notes", "000002_note_search", "000003_catalogs", "000004_note_places", "000005_users_authors_sessions")

	if _, err := db.ExecContext(
		ctx,
		`
			INSERT INTO notes (id, title, body, category_slug, place_slug, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
		"existing-text-note",
		"Text-only note",
		"Survives image migration.",
		"food",
		"sao-paulo",
		int64(1782993600000),
		int64(1782993600000),
	); err != nil {
		t.Fatalf("insert existing text-only note: %v", err)
	}

	applyMigrationHistorySchema(t, ctx, db, "000006_note_ownership", "000007_author_notes_index", "000008_note_cursor_invariants")

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply remaining migrations: %v", err)
	}

	var imageRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_images WHERE note_id = ?`, "existing-text-note").Scan(&imageRows); err != nil {
		t.Fatalf("query migrated note images: %v", err)
	}
	if imageRows != 0 {
		t.Fatalf("migrated text-only note image rows = %d, want 0", imageRows)
	}

	store := NewNoteStore(db)
	assertEmptyImages := func(label string, found note.Note) {
		t.Helper()
		if found.ID != "existing-text-note" {
			t.Fatalf("%s note id = %q, want existing-text-note", label, found.ID)
		}
		if found.Images == nil || len(found.Images) != 0 {
			t.Fatalf("%s note images = %#v, want non-nil empty", label, found.Images)
		}
	}
	reads := []struct {
		name string
		read func() ([]note.Note, error)
	}{
		{
			name: "recent",
			read: func() ([]note.Note, error) {
				return store.ListRecentNotes(ctx, note.ListInput{Limit: 10})
			},
		},
		{
			name: "search",
			read: func() ([]note.Note, error) {
				return store.SearchNotes(ctx, note.SearchInput{Query: "survives", Limit: 10})
			},
		},
	}
	for _, test := range reads {
		t.Run(test.name, func(t *testing.T) {
			found, err := test.read()
			if err != nil {
				t.Fatalf("read migrated note: %v", err)
			}
			if len(found) != 1 {
				t.Fatalf("read migrated note count = %d, want 1", len(found))
			}
			assertEmptyImages(test.name, found[0])
		})
	}

	found, err := store.FindNote(ctx, "existing-text-note", systemNoteOwnerUserID)
	if err != nil {
		t.Fatalf("find migrated note: %v", err)
	}
	assertEmptyImages("detail", found)

	authorPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{
		AuthorID: systemNoteOwnerAuthorID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list migrated author notes: %v", err)
	}
	if len(authorPage.Notes) != 1 {
		t.Fatalf("migrated author note count = %d, want 1", len(authorPage.Notes))
	}
	assertEmptyImages("author", authorPage.Notes[0].Note)
}

// applyMigrationHistorySchema builds a deliberately partial schema by applying
// only the named migration files. It is for migration-history assertions only
// and MUST NOT be used by repository behavior tests, which use
// openMigratedDatabase to apply the full history.
func applyMigrationHistorySchema(t *testing.T, ctx context.Context, db *sql.DB, versions ...string) {
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

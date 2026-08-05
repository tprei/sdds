package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	deleteTestUserA = "usr-delete-a"
	deleteTestUserB = "usr-delete-b"
	deleteTestNoteA = "note-delete-a"
	deleteTestNoteB = "note-delete-b"
)

func seedDeleteTestUser(t *testing.T, ctx context.Context, db *sql.DB, userID, authorID, loginID, sessionID, username string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, userID); err != nil {
		t.Fatalf("insert user %s: %v", userID, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO authors (id, user_id, display_name, created_at, updated_at) VALUES (?, ?, ?, 0, 0)`, authorID, userID, username); err != nil {
		t.Fatalf("insert author %s: %v", authorID, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
		VALUES (?, ?, 'password', 'local', ?, ?, 0, 0)`, loginID, userID, username, "hash-"+username); err != nil {
		t.Fatalf("insert login identity %s: %v", loginID, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, 0, 0)`, sessionID, userID, "token-"+username); err != nil {
		t.Fatalf("insert session %s: %v", sessionID, err)
	}
}

func seedDeleteTestNote(t *testing.T, ctx context.Context, db *sql.DB, noteID, userID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO notes (id, user_id, title, body, category_slug, created_at, updated_at)
		VALUES (?, ?, 'Nota', 'corpo', 'food', 1, 1)`, noteID, userID); err != nil {
		t.Fatalf("insert note %s: %v", noteID, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO note_search (note_id, title, body) VALUES (?, 'Nota', 'corpo')`, noteID); err != nil {
		t.Fatalf("insert note_search %s: %v", noteID, err)
	}
}

func countDeleteRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count %q args %v: %v", query, args, err)
	}
	return count
}

func TestUserStoreDeleteUserPurgesEveryPersonalRow(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	seedDeleteTestUser(t, ctx, db, deleteTestUserA, "auth-a", "login-a", "sess-a", "usuarioa")
	seedDeleteTestUser(t, ctx, db, deleteTestUserB, "auth-b", "login-b", "sess-b", "usuariob")
	seedDeleteTestNote(t, ctx, db, deleteTestNoteA, deleteTestUserA)
	seedDeleteTestNote(t, ctx, db, deleteTestNoteB, deleteTestUserB)

	storageKey := media.ObjectKey("note-images/upload-a")
	insertImageUploadRow(t, db, media.PendingInput{
		ID:                    "upload-a",
		UserID:                deleteTestUserA,
		StorageKey:            storageKey,
		UploadRequestID:       "req-a",
		ContentType:           "image/jpeg",
		ByteSize:              99,
		Width:                 1,
		Height:                1,
		SHA256:                strings.Repeat("a", 64),
		CreatedAt:             time.UnixMilli(1),
		UpdatedAt:             time.UnixMilli(1),
		WriteLeaseUntil:       time.UnixMilli(2),
		ExpiresAt:             time.UnixMilli(3),
		RequestRetentionUntil: time.UnixMilli(4),
	}, "consumed", deleteTestNoteA, nil)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO note_comments (id, note_id, user_id, body, created_at)
		VALUES ('comment-a-on-b', ?, ?, 'comentário', 1)`, deleteTestNoteB, deleteTestUserA); err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO note_useful_reactions (note_id, user_id, created_at)
		VALUES (?, ?, 1)`, deleteTestNoteB, deleteTestUserA); err != nil {
		t.Fatalf("insert useful: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, created_at)
		VALUES ('report-a', ?, 'note', ?, 'spam', 1)`, deleteTestUserA, deleteTestNoteB); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO events (id, kind, occurred_at, received_at, user_id, app_platform, schema_version, payload_json)
		VALUES ('event-a', 'note_published', 1, 1, ?, 'web', 1, '{}')`, deleteTestUserA); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, created_at, updated_at)
		VALUES ('channel-a', ?, 'email', 'a@example.com', 1, 1)`, deleteTestUserA); err != nil {
		t.Fatalf("insert contact channel: %v", err)
	}

	store := NewUserStore(db)
	deletedAt := time.UnixMilli(500)
	if err := store.DeleteUser(ctx, user.UserID(deleteTestUserA), deletedAt); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	cases := []struct {
		label string
		query string
		args  []any
		want  int
	}{
		{"user a gone", `SELECT COUNT(*) FROM users WHERE id = ?`, []any{deleteTestUserA}, 0},
		{"user b survives", `SELECT COUNT(*) FROM users WHERE id = ?`, []any{deleteTestUserB}, 1},
		{"author a gone", `SELECT COUNT(*) FROM authors WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"author b survives", `SELECT COUNT(*) FROM authors WHERE user_id = ?`, []any{deleteTestUserB}, 1},
		{"login identity a gone", `SELECT COUNT(*) FROM user_login_identities WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"session a gone", `SELECT COUNT(*) FROM sessions WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"note a gone", `SELECT COUNT(*) FROM notes WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"note b survives", `SELECT COUNT(*) FROM notes WHERE id = ?`, []any{deleteTestNoteB}, 1},
		{"note_search a gone", `SELECT COUNT(*) FROM note_search WHERE note_id = ?`, []any{deleteTestNoteA}, 0},
		{"note_search b survives", `SELECT COUNT(*) FROM note_search WHERE note_id = ?`, []any{deleteTestNoteB}, 1},
		{"image upload a gone", `SELECT COUNT(*) FROM image_uploads WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"comment by a gone", `SELECT COUNT(*) FROM note_comments WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"useful by a gone", `SELECT COUNT(*) FROM note_useful_reactions WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"report by a gone", `SELECT COUNT(*) FROM reports WHERE reporter_user_id = ?`, []any{deleteTestUserA}, 0},
		{"event by a gone", `SELECT COUNT(*) FROM events WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"contact channel a gone", `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = ?`, []any{deleteTestUserA}, 0},
		{"orphan queue carries storage key", `SELECT COUNT(*) FROM orphaned_media_objects WHERE storage_key = ?`, []any{string(storageKey)}, 1},
	}
	for _, c := range cases {
		if got := countDeleteRows(t, db, c.query, c.args...); got != c.want {
			t.Fatalf("%s = %d, want %d", c.label, got, c.want)
		}
	}
}

func TestUserStoreDeleteUserRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	seedDeleteTestUser(t, ctx, db, deleteTestUserA, "auth-a", "login-a", "sess-a", "usuarioa")
	seedDeleteTestNote(t, ctx, db, deleteTestNoteA, deleteTestUserA)
	insertImageUploadRow(t, db, media.PendingInput{
		ID:                    "upload-a",
		UserID:                deleteTestUserA,
		StorageKey:            media.ObjectKey("note-images/upload-a"),
		UploadRequestID:       "req-a",
		ContentType:           "image/jpeg",
		ByteSize:              1,
		Width:                 1,
		Height:                1,
		SHA256:                strings.Repeat("a", 64),
		CreatedAt:             time.UnixMilli(1),
		UpdatedAt:             time.UnixMilli(1),
		ExpiresAt:             time.UnixMilli(3),
		RequestRetentionUntil: time.UnixMilli(4),
	}, "consumed", deleteTestNoteA, nil)

	// Drop the queue table so the second statement inside the purge transaction
	// fails. The transaction must roll back and leave every seeded row intact.
	if _, err := db.Exec(`DROP TABLE orphaned_media_objects`); err != nil {
		t.Fatalf("drop orphan table: %v", err)
	}

	store := NewUserStore(db)
	err := store.DeleteUser(ctx, user.UserID(deleteTestUserA), time.UnixMilli(500))
	if err == nil {
		t.Fatalf("delete user: expected failure when the orphan queue is unavailable")
	}

	cases := []struct {
		label string
		query string
		args  []any
		want  int
	}{
		{"user intact", `SELECT COUNT(*) FROM users WHERE id = ?`, []any{deleteTestUserA}, 1},
		{"note intact", `SELECT COUNT(*) FROM notes WHERE id = ?`, []any{deleteTestNoteA}, 1},
		{"note_search intact", `SELECT COUNT(*) FROM note_search WHERE note_id = ?`, []any{deleteTestNoteA}, 1},
		{"image upload intact", `SELECT COUNT(*) FROM image_uploads WHERE user_id = ?`, []any{deleteTestUserA}, 1},
	}
	for _, c := range cases {
		if got := countDeleteRows(t, db, c.query, c.args...); got != c.want {
			t.Fatalf("rollback: %s = %d, want %d", c.label, got, c.want)
		}
	}
}

func TestUserStoreDeleteMissingUserIsNotAnError(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewUserStore(db)
	if err := store.DeleteUser(ctx, user.UserID("never-existed"), time.UnixMilli(1)); err != nil {
		t.Fatalf("delete missing user: unexpected error %v", err)
	}
}

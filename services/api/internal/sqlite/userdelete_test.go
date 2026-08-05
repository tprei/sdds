package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

func countUserDeleteRows(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows (%s): %v", query, err)
	}
	return count
}

func assertUserDeleteRowsGone(t *testing.T, ctx context.Context, db *sql.DB, label, query string, args ...any) {
	t.Helper()
	if count := countUserDeleteRows(t, ctx, db, query, args...); count != 0 {
		t.Fatalf("%s count = %d, want 0", label, count)
	}
}

// seedAccountToDelete creates a full user (login identity, author, session, a
// note with an image and comment, a useful row, an event, a report they filed,
// a report someone else filed against their note, a contact channel with a
// token, and a comment they left on someone else's note) and returns the IDs
// the tests assert against. A second user owns the note the deleted user
// commented on.
type seededAccount struct {
	userID        user.UserID
	noteID        string
	otherNoteID   string
	otherUserID   user.UserID
	uploadStorage media.ObjectKey
}

func seedAccountToDelete(t *testing.T, ctx context.Context, db *sql.DB, now time.Time) seededAccount {
	t.Helper()
	userStore := newUserStore(db, func() time.Time { return now })
	noteStore := newNoteStore(db, func() time.Time { return now })
	commentStore := NewCommentStore(db)

	mainSession, err := userStore.CreatePasswordUser(ctx, user.CreatePasswordUserInput{
		Username: "delete-me", DisplayName: "Apagar",
		SecretHash: "password-hash", TokenHash: user.HashSessionToken("delete-me-token"),
		ExpiresAt: now.Add(user.SessionLifetime),
	})
	if err != nil {
		t.Fatalf("create main user: %v", err)
	}
	otherSession, err := userStore.CreatePasswordUser(ctx, user.CreatePasswordUserInput{
		Username: "keep-me", DisplayName: "Manter",
		SecretHash: "password-hash", TokenHash: user.HashSessionToken("keep-me-token"),
		ExpiresAt: now.Add(user.SessionLifetime),
	})
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	created, err := noteStore.CreateNote(ctx, note.CreateInput{
		UserID: mainSession.User.ID, ClientRequestID: "delete-me-note",
		Title: "Nota pra apagar", Body: "Some embora com a conta.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create main note: %v", err)
	}
	otherNote, err := noteStore.CreateNote(ctx, note.CreateInput{
		UserID: otherSession.User.ID, ClientRequestID: "keep-me-note",
		Title: "Nota que fica", Body: "Fica no feed.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create other note: %v", err)
	}

	// An image attached to the main note + its consumed upload.
	if _, err := db.ExecContext(ctx, noteImageInsertSQL, "delete-me-image", created.ID,
		"note-images/delete-me", "image/jpeg", 10, 100, 80, strings.Repeat("a", 64), 0, 0, 0); err != nil {
		t.Fatalf("insert note image: %v", err)
	}
	upload := imageUploadInput(now, "delete-me-upload", "delete-request", string(mainSession.User.ID), 10)
	insertImageUploadRow(t, db, upload, string(media.UploadConsumed), created.ID, nil)

	// The user's useful row on the other note (deleted via user_id cascade), the
	// other user's useful on the main note (deleted via note_id cascade), and the
	// other user's useful on their own note (survives, proving content is intact).
	if _, err := db.ExecContext(ctx, `INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`,
		otherNote.ID, mainSession.User.ID, now.UnixMilli()); err != nil {
		t.Fatalf("insert main useful: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`,
		created.ID, otherSession.User.ID, now.UnixMilli()); err != nil {
		t.Fatalf("insert other useful on main note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`,
		otherNote.ID, otherSession.User.ID, now.UnixMilli()); err != nil {
		t.Fatalf("insert other own useful: %v", err)
	}

	// A comment on the user's own note and one on the other user's note.
	if _, err := commentStore.CreateComment(ctx, comment.CreateInput{
		NoteID: created.ID, UserID: mainSession.User.ID, Body: "Comentario proprio",
	}); err != nil {
		t.Fatalf("create own comment: %v", err)
	}
	if _, err := commentStore.CreateComment(ctx, comment.CreateInput{
		NoteID: otherNote.ID, UserID: mainSession.User.ID, Body: "Comentario alheio",
	}); err != nil {
		t.Fatalf("create other-note comment: %v", err)
	}

	// An event attributed to the user, and a report the user filed.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO events (id, kind, occurred_at, received_at, user_id, app_platform, schema_version, payload_json)
		VALUES ('delete-me-event', 'note_published', ?, ?, ?, 'web', 1, '{}')`,
		now.UnixMilli(), now.UnixMilli(), mainSession.User.ID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, created_at)
		VALUES ('delete-me-report', ?, 'note', ?, 'spam', ?)`,
		mainSession.User.ID, otherNote.ID, now.UnixMilli()); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	// A report the other user filed against the deleted user's note: it must
	// survive deletion as moderation history with a missing target.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO reports (id, reporter_user_id, target_type, target_id, reason, created_at)
		VALUES ('keep-me-report', ?, 'note', ?, 'spam', ?)`,
		otherSession.User.ID, created.ID, now.UnixMilli()); err != nil {
		t.Fatalf("insert keep report: %v", err)
	}

	// A contact channel and token for the user.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, created_at, updated_at)
		VALUES ('delete-me-channel', ?, 'email', 'apagar@example.com', ?, ?)`,
		mainSession.User.ID, now.UnixMilli(), now.UnixMilli()); err != nil {
		t.Fatalf("insert contact channel: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_contact_channel_tokens (id, channel_id, purpose, token_hash, created_at, expires_at, consumed_at)
		VALUES ('delete-me-token', 'delete-me-channel', 'verify', 'hash-delete', ?, ?, NULL)`,
		now.UnixMilli(), now.Add(time.Hour).UnixMilli()); err != nil {
		t.Fatalf("insert contact token: %v", err)
	}

	return seededAccount{
		userID: mainSession.User.ID, noteID: created.ID,
		otherNoteID: otherNote.ID, otherUserID: otherSession.User.ID,
		uploadStorage: upload.StorageKey,
	}
}

// TestDeleteUserRemovesEveryPersonalRow proves the account delete cascade
// reaches every personal table while leaving another user's content intact.
func TestDeleteUserRemovesEveryPersonalRow(t *testing.T) {
	ctx := context.Background()
	now := time.UnixMilli(9_000_000).UTC()
	db := openMigratedDatabase(t, ctx)
	seeded := seedAccountToDelete(t, ctx, db, now)

	userStore := newUserStore(db, func() time.Time { return now })
	if err := userStore.DeleteUser(ctx, seeded.userID, now); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	uid := string(seeded.userID)
	for _, check := range []struct {
		label string
		query string
	}{
		{"users", `SELECT COUNT(*) FROM users WHERE id = ?`},
		{"authors", `SELECT COUNT(*) FROM authors WHERE user_id = ?`},
		{"sessions", `SELECT COUNT(*) FROM sessions WHERE user_id = ?`},
		{"login identities", `SELECT COUNT(*) FROM user_login_identities WHERE user_id = ?`},
		{"notes", `SELECT COUNT(*) FROM notes WHERE user_id = ?`},
		{"note_comments", `SELECT COUNT(*) FROM note_comments WHERE user_id = ?`},
		{"note_useful_reactions by user", `SELECT COUNT(*) FROM note_useful_reactions WHERE user_id = ?`},
		{"note_create_requests", `SELECT COUNT(*) FROM note_create_requests WHERE user_id = ?`},
		{"events", `SELECT COUNT(*) FROM events WHERE user_id = ?`},
		{"reports filed", `SELECT COUNT(*) FROM reports WHERE reporter_user_id = ?`},
		{"contact channels", `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = ?`},
		{"contact tokens", `SELECT COUNT(*) FROM user_contact_channel_tokens WHERE channel_id IN (SELECT id FROM user_contact_channels WHERE user_id = ?)`},
	} {
		assertUserDeleteRowsGone(t, ctx, db, check.label, check.query, uid)
	}
	// The note-scoped checks target the deleted note specifically.
	assertUserDeleteRowsGone(t, ctx, db, "note_search for note", `SELECT COUNT(*) FROM note_search WHERE note_id = ?`, seeded.noteID)
	assertUserDeleteRowsGone(t, ctx, db, "note_images for note", `SELECT COUNT(*) FROM note_images WHERE note_id = ?`, seeded.noteID)
	assertUserDeleteRowsGone(t, ctx, db, "note_embeddings for note", `SELECT COUNT(*) FROM note_embeddings WHERE note_id = ?`, seeded.noteID)
	assertUserDeleteRowsGone(t, ctx, db, "note_useful_reactions on deleted note", `SELECT COUNT(*) FROM note_useful_reactions WHERE note_id = ?`, seeded.noteID)

	// The other user's content is untouched.
	if count := countUserDeleteRows(t, ctx, db, `SELECT COUNT(*) FROM users WHERE id = ?`, string(seeded.otherUserID)); count != 1 {
		t.Fatalf("other user count = %d, want 1", count)
	}
	if count := countUserDeleteRows(t, ctx, db, `SELECT COUNT(*) FROM notes WHERE id = ?`, seeded.otherNoteID); count != 1 {
		t.Fatalf("other note count = %d, want 1", count)
	}
	if count := countUserDeleteRows(t, ctx, db, `SELECT COUNT(*) FROM note_useful_reactions WHERE note_id = ? AND user_id = ?`, seeded.otherNoteID, string(seeded.otherUserID)); count != 1 {
		t.Fatalf("other user useful on their own note count = %d, want 1", count)
	}

	// Moderation history outlives the account: reports carry no foreign key to
	// their target, so the report about the deleted note stays behind.
	if count := countUserDeleteRows(t, ctx, db, `SELECT COUNT(*) FROM reports WHERE id = ?`, "keep-me-report"); count != 1 {
		t.Fatalf("report about the deleted note count = %d, want 1", count)
	}
}

// TestDeleteUserLeavesImageUploadsClaimable proves the deleted user's uploads
// survive as "deleting" rows the retention sweep reclaims, rather than being
// dropped (which would orphan the object bytes).
func TestDeleteUserLeavesImageUploadsClaimable(t *testing.T) {
	ctx := context.Background()
	now := time.UnixMilli(9_000_000).UTC()
	db := openMigratedDatabase(t, ctx)
	seeded := seedAccountToDelete(t, ctx, db, now)

	userStore := newUserStore(db, func() time.Time { return now })
	if err := userStore.DeleteUser(ctx, seeded.userID, now); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var state string
	var uploadUserID sql.NullString
	var lease any
	var consumedNoteID any
	if err := db.QueryRowContext(ctx,
		`SELECT state, user_id, write_lease_until, consumed_note_id FROM image_uploads WHERE storage_key = ?`,
		seeded.uploadStorage,
	).Scan(&state, &uploadUserID, &lease, &consumedNoteID); err != nil {
		t.Fatalf("read upload row: %v", err)
	}
	if state != string(media.UploadDeleting) {
		t.Fatalf("upload state = %q, want %q", state, media.UploadDeleting)
	}
	if uploadUserID.Valid {
		t.Fatalf("upload user_id = %q, want NULL", uploadUserID.String)
	}
	if lease != nil {
		t.Fatalf("upload write_lease_until = %v, want nil", lease)
	}
	if consumedNoteID != nil {
		t.Fatalf("upload consumed_note_id = %v, want nil", consumedNoteID)
	}

	uploadStore := newImageUploadStore(db, func() time.Time { return now })
	claimed, err := uploadStore.ClaimExpired(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim expired: %v", err)
	}
	found := false
	for _, c := range claimed {
		if c.StorageKey == seeded.uploadStorage {
			found = true
		}
	}
	if !found {
		t.Fatalf("orphaned upload %q was not claimed by the retention sweep", seeded.uploadStorage)
	}
}

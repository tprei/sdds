package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	usefulStoreMarkerUserID   = user.UserID("018ff5b8-0000-7000-8000-000000000001")
	usefulStoreMarkerAuthorID = author.AuthorID("018ff5b8-0000-7000-8000-000000000002")
	usefulStoreOtherUserID    = user.UserID("018ff5b8-0000-7000-8000-000000000003")
	usefulStoreSecondUserID   = user.UserID("018ff5b8-0000-7000-8000-000000000004")
	usefulStoreBareUserID     = user.UserID("018ff5b8-0000-7000-8000-000000000005")
)

func newUsefulStoreTestStore(t *testing.T, ctx context.Context) (*NoteStore, *sql.DB) {
	t.Helper()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newNoteStore(db, func() time.Time { return now })
	return store, db
}

func insertUsefulStoreUser(t *testing.T, ctx context.Context, db execer, userID user.UserID, authorID author.AuthorID, displayName string) {
	t.Helper()
	insertAuthorStoreUser(t, ctx, db, userID, authorID, displayName)
}

func insertBareUsefulStoreUser(t *testing.T, ctx context.Context, db execer, userID user.UserID) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, userID); err != nil {
		t.Fatalf("insert bare user %s: %v", userID, err)
	}
}

func countUsefulReactions(t *testing.T, ctx context.Context, db *sql.DB, noteID string, userID user.UserID) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM note_useful_reactions WHERE note_id = ? AND user_id = ?`,
		noteID,
		string(userID),
	).Scan(&count); err != nil {
		t.Fatalf("count useful reactions: %v", err)
	}
	return count
}

func TestUsefulStoreMarkIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)

	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreMarkerUserID}); err != nil {
		t.Fatalf("first mark: %v", err)
	}
	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreMarkerUserID}); err != nil {
		t.Fatalf("repeat mark: %v", err)
	}

	if got := countUsefulReactions(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID); got != 1 {
		t.Fatalf("reaction count = %d, want 1", got)
	}
}

func TestUsefulStorePrimaryKeyRejectsDirectDuplicate(t *testing.T) {
	ctx := context.Background()
	_, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`,
		"018ff5b8-0000-7000-8000-000000000010",
		string(usefulStoreMarkerUserID),
		1782993600000,
	); err != nil {
		t.Fatalf("first direct insert: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`,
		"018ff5b8-0000-7000-8000-000000000010",
		string(usefulStoreMarkerUserID),
		1782993600001,
	); err == nil {
		t.Fatal("duplicate direct insert succeeded, want primary key violation")
	}
}

func TestUsefulStoreUnmarkIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)

	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreMarkerUserID}); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if err := store.UnmarkUseful(ctx, note.UnmarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreMarkerUserID}); err != nil {
		t.Fatalf("first unmark: %v", err)
	}
	if err := store.UnmarkUseful(ctx, note.UnmarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreMarkerUserID}); err != nil {
		t.Fatalf("repeat unmark: %v", err)
	}

	if got := countUsefulReactions(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID); got != 0 {
		t.Fatalf("reaction count = %d, want 0", got)
	}
}

func TestUsefulStoreReactionsAreIsolatedPerUserAndNote(t *testing.T) {
	ctx := context.Background()
	store, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000011", usefulStoreMarkerUserID, 1782993601000)
	insertBareUsefulStoreUser(t, ctx, db, usefulStoreOtherUserID)
	insertBareUsefulStoreUser(t, ctx, db, usefulStoreSecondUserID)

	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreOtherUserID}); err != nil {
		t.Fatalf("mark first note first user: %v", err)
	}
	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreSecondUserID}); err != nil {
		t.Fatalf("mark first note second user: %v", err)
	}
	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000011", UserID: usefulStoreOtherUserID}); err != nil {
		t.Fatalf("mark second note first user: %v", err)
	}

	cases := []struct {
		name   string
		noteID string
		userID user.UserID
		want   int
	}{
		{name: "first note first user", noteID: "018ff5b8-0000-7000-8000-000000000010", userID: usefulStoreOtherUserID, want: 1},
		{name: "first note second user", noteID: "018ff5b8-0000-7000-8000-000000000010", userID: usefulStoreSecondUserID, want: 1},
		{name: "second note first user", noteID: "018ff5b8-0000-7000-8000-000000000011", userID: usefulStoreOtherUserID, want: 1},
		{name: "second note second user", noteID: "018ff5b8-0000-7000-8000-000000000011", userID: usefulStoreSecondUserID, want: 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := countUsefulReactions(t, ctx, db, tt.noteID, tt.userID); got != tt.want {
				t.Fatalf("reaction count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUsefulStoreCascadesOnNoteDeletion(t *testing.T) {
	ctx := context.Background()
	store, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)
	insertBareUsefulStoreUser(t, ctx, db, usefulStoreOtherUserID)
	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreOtherUserID}); err != nil {
		t.Fatalf("mark: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, "018ff5b8-0000-7000-8000-000000000010"); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	if got := countUsefulReactions(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreOtherUserID); got != 0 {
		t.Fatalf("reaction count after note delete = %d, want 0", got)
	}
}

func TestUsefulStoreCascadesOnBareUserDeletion(t *testing.T) {
	ctx := context.Background()
	store, db := newUsefulStoreTestStore(t, ctx)
	insertUsefulStoreUser(t, ctx, db, usefulStoreMarkerUserID, usefulStoreMarkerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreMarkerUserID, 1782993600000)
	insertBareUsefulStoreUser(t, ctx, db, usefulStoreBareUserID)
	if err := store.MarkUseful(ctx, note.MarkUsefulInput{NoteID: "018ff5b8-0000-7000-8000-000000000010", UserID: usefulStoreBareUserID}); err != nil {
		t.Fatalf("mark: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, usefulStoreBareUserID); err != nil {
		t.Fatalf("delete bare marker user: %v", err)
	}

	if got := countUsefulReactions(t, ctx, db, "018ff5b8-0000-7000-8000-000000000010", usefulStoreBareUserID); got != 0 {
		t.Fatalf("reaction count after user delete = %d, want 0", got)
	}
}

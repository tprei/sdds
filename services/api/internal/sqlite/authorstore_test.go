package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	authorStoreUserID        = user.UserID("018ff5b8-0000-7000-8000-000000000101")
	authorStoreAuthorID      = user.AuthorID("018ff5b8-0000-7000-8000-000000000102")
	otherAuthorStoreUserID   = user.UserID("018ff5b8-0000-7000-8000-000000000201")
	otherAuthorStoreAuthorID = user.AuthorID("018ff5b8-0000-7000-8000-000000000202")
)

func TestUserStoreFindsPublicAuthorWithNoteCount(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreUser(t, ctx, db, otherAuthorStoreUserID, otherAuthorStoreAuthorID, "João Silva")
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000301", authorStoreUserID, 1782993600000)
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000302", authorStoreUserID, 1782993601000)
	insertAuthorStoreNote(t, ctx, db, "018ff5b8-0000-7000-8000-000000000303", otherAuthorStoreUserID, 1782993602000)

	found, err := NewUserStore(db).FindPublicAuthor(ctx, authorStoreAuthorID)
	if err != nil {
		t.Fatalf("find public author: %v", err)
	}
	want := user.PublicAuthor{ID: authorStoreAuthorID, DisplayName: "Marina Alves", NoteCount: 2}
	if diff := cmp.Diff(want, found); diff != "" {
		t.Fatalf("public author mismatch (-want +got):\n%s", diff)
	}
}

func TestUserStoreFindsPublicAuthorWithZeroNotes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")

	found, err := NewUserStore(db).FindPublicAuthor(ctx, authorStoreAuthorID)
	if err != nil {
		t.Fatalf("find public author: %v", err)
	}
	if found.NoteCount != 0 {
		t.Fatalf("note count = %d, want 0", found.NoteCount)
	}
}

func TestUserStoreFindsUnknownPublicAuthorAsNotFound(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	_, err := NewUserStore(db).FindPublicAuthor(ctx, user.AuthorID("018ff5b8-0000-7000-8000-000000000999"))
	if !errors.Is(err, user.ErrAuthorNotFound) {
		t.Fatalf("find public author error = %v, want ErrAuthorNotFound", err)
	}
}

func TestNoteStoreListsAuthorNotesWithKeysetPagination(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreUser(t, ctx, db, otherAuthorStoreUserID, otherAuthorStoreAuthorID, "João Silva")

	sameTime := int64(1782993600000)
	olderTime := int64(1782993599000)
	newerOtherAuthorTime := int64(1782993605000)
	newerTieID := "018ff5b8-0000-7000-8000-000000000503"
	olderTieID := "018ff5b8-0000-7000-8000-000000000502"
	oldestID := "018ff5b8-0000-7000-8000-000000000501"
	otherAuthorID := "018ff5b8-0000-7000-8000-000000000599"
	insertAuthorStoreNote(t, ctx, db, olderTieID, authorStoreUserID, sameTime)
	insertAuthorStoreNote(t, ctx, db, newerTieID, authorStoreUserID, sameTime)
	insertAuthorStoreNote(t, ctx, db, oldestID, authorStoreUserID, olderTime)
	insertAuthorStoreNote(t, ctx, db, otherAuthorID, otherAuthorStoreUserID, newerOtherAuthorTime)

	store := NewNoteStore(db)
	firstPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{AuthorID: authorStoreAuthorID, Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if !firstPage.HasMore {
		t.Fatal("first page HasMore = false, want true")
	}
	wantFirstIDs := []string{newerTieID, olderTieID}
	if diff := cmp.Diff(wantFirstIDs, noteIDs(firstPage.Notes)); diff != "" {
		t.Fatalf("first page ids mismatch (-want +got):\n%s", diff)
	}

	secondPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{
		AuthorID: authorStoreAuthorID,
		Limit:    2,
		After:    &note.AuthorNotePosition{CreatedAt: time.UnixMilli(sameTime).UTC(), ID: olderTieID},
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if secondPage.HasMore {
		t.Fatal("second page HasMore = true, want false")
	}
	wantSecondIDs := []string{oldestID}
	if diff := cmp.Diff(wantSecondIDs, noteIDs(secondPage.Notes)); diff != "" {
		t.Fatalf("second page ids mismatch (-want +got):\n%s", diff)
	}

	terminalPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{
		AuthorID: authorStoreAuthorID,
		Limit:    2,
		After:    &note.AuthorNotePosition{CreatedAt: time.UnixMilli(olderTime).UTC(), ID: oldestID},
	})
	if err != nil {
		t.Fatalf("list terminal page: %v", err)
	}
	if terminalPage.HasMore {
		t.Fatal("terminal page HasMore = true, want false")
	}
	if len(terminalPage.Notes) != 0 {
		t.Fatalf("terminal note count = %d, want 0", len(terminalPage.Notes))
	}
}

func insertAuthorStoreUser(t *testing.T, ctx context.Context, db execer, userID user.UserID, authorID user.AuthorID, displayName string) {
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
		`INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		userID,
		"Café bom",
		"Tem pão de queijo decente.",
		note.CategorySlugFood,
		note.PlaceSlugSaoPaulo,
		createdAt,
		createdAt,
	); err != nil {
		t.Fatalf("insert note %s: %v", id, err)
	}
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

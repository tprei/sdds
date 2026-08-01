package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	commentStoreOwnerUserID   = user.UserID("018ff5b8-0000-7000-8000-000000000101")
	commentStoreOwnerAuthorID = author.AuthorID("018ff5b8-0000-7000-8000-000000000102")
	commentStoreOtherUserID   = user.UserID("018ff5b8-0000-7000-8000-000000000103")
	commentStoreOtherAuthorID = author.AuthorID("018ff5b8-0000-7000-8000-000000000104")
	commentStoreBareUserID    = user.UserID("018ff5b8-0000-7000-8000-000000000105")
	commentStoreNoteID        = "018ff5b8-0000-7000-8000-000000000110"
	commentStoreOtherNoteID   = "018ff5b8-0000-7000-8000-000000000111"
)

func TestCommentStoreCreatesAndFindsJoinedComment(t *testing.T) {
	ctx := context.Background()
	store, db, now := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	created := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, " \n comentário 😀 \t")
	if created.Body != "comentário 😀" {
		t.Fatalf("created body = %q, want %q", created.Body, "comentário 😀")
	}
	if created.NoteID != commentStoreNoteID || created.UserID != commentStoreOwnerUserID {
		t.Fatalf("created identity = (%q, %q), want (%q, %q)", created.NoteID, created.UserID, commentStoreNoteID, commentStoreOwnerUserID)
	}
	if diff := cmp.Diff(comment.AuthorSummary{ID: commentStoreOwnerAuthorID, DisplayName: "Marina Alves"}, created.Author); diff != "" {
		t.Fatalf("created author mismatch (-want +got):\n%s", diff)
	}
	wantCreatedAt := time.UnixMilli(now.UnixMilli()).UTC()
	if created.CreatedAt != wantCreatedAt {
		t.Fatalf("created_at = %s, want %s", created.CreatedAt, wantCreatedAt)
	}
	parsedID, err := uuid.Parse(string(created.ID))
	if err != nil {
		t.Fatalf("parse created id: %v", err)
	}
	if parsedID.Version() != 7 {
		t.Fatalf("created id version = %d, want 7", parsedID.Version())
	}

	if _, err := db.ExecContext(ctx, `UPDATE authors SET display_name = ? WHERE id = ?`, "Marina Atualizada", commentStoreOwnerAuthorID); err != nil {
		t.Fatalf("update author: %v", err)
	}
	found, err := store.FindComment(ctx, commentStoreNoteID, string(created.ID))
	if err != nil {
		t.Fatalf("find comment: %v", err)
	}
	if found.Author.DisplayName != "Marina Atualizada" {
		t.Fatalf("found display name = %q, want %q", found.Author.DisplayName, "Marina Atualizada")
	}
}

func TestCommentStoreListsOldestFirstWithKeysetPagination(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertCommentStoreAuthor(t, ctx, db, commentStoreOtherUserID, commentStoreOtherAuthorID, "João Lima")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	insertAuthorStoreNote(t, ctx, db, commentStoreOtherNoteID, commentStoreOwnerUserID, 0)

	first := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "primeiro")
	second := createComment(t, ctx, store, commentStoreNoteID, commentStoreOtherUserID, "segundo")
	third := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "terceiro")
	otherNote := createComment(t, ctx, store, commentStoreOtherNoteID, commentStoreOtherUserID, "outra nota")

	firstPage, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if !firstPage.HasMore {
		t.Fatal("first page HasMore = false, want true")
	}
	if diff := cmp.Diff([]comment.CommentID{first.ID, second.ID}, commentIDs(firstPage.Comments)); diff != "" {
		t.Fatalf("first page ids mismatch (-want +got):\n%s", diff)
	}

	secondPage, err := store.ListNoteComments(ctx, comment.ListInput{
		NoteID: commentStoreNoteID,
		Limit:  2,
		After:  &firstPage.Comments[1].Position,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if secondPage.HasMore {
		t.Fatal("second page HasMore = true, want false")
	}
	if diff := cmp.Diff([]comment.CommentID{third.ID}, commentIDs(secondPage.Comments)); diff != "" {
		t.Fatalf("second page ids mismatch (-want +got):\n%s", diff)
	}

	otherPage, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreOtherNoteID})
	if err != nil {
		t.Fatalf("list other note: %v", err)
	}
	if diff := cmp.Diff([]comment.CommentID{otherNote.ID}, commentIDs(otherPage.Comments)); diff != "" {
		t.Fatalf("other note ids mismatch (-want +got):\n%s", diff)
	}

	empty, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: "missing-note"})
	if err != nil {
		t.Fatalf("list missing note comments: %v", err)
	}
	if empty.Comments == nil || len(empty.Comments) != 0 || empty.HasMore {
		t.Fatalf("empty page = %#v, want non-nil empty comments and HasMore false", empty)
	}
}

func TestCommentStoreFindAndDeleteReturnNotFound(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	created := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "comentário")

	if _, err := store.FindComment(ctx, commentStoreNoteID, "missing"); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("find missing error = %v, want ErrCommentNotFound", err)
	}
	if _, err := store.FindComment(ctx, "wrong-note", string(created.ID)); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("find under wrong note error = %v, want ErrCommentNotFound", err)
	}
	if err := store.DeleteComment(ctx, string(created.ID)); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	if _, err := store.FindComment(ctx, commentStoreNoteID, string(created.ID)); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("find deleted error = %v, want ErrCommentNotFound", err)
	}
	if err := store.DeleteComment(ctx, string(created.ID)); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("repeat delete error = %v, want ErrCommentNotFound", err)
	}
}

func TestCommentStoreEnforcesInputConstraintsAndCascades(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	insertAuthorStoreNote(t, ctx, db, commentStoreOtherNoteID, commentStoreOwnerUserID, 0)

	for _, body := range []string{"", " \t\n", strings.Repeat("😀", comment.BodyMaxLength+1)} {
		if _, err := store.CreateComment(ctx, comment.CreateInput{NoteID: commentStoreNoteID, UserID: commentStoreOwnerUserID, Body: body}); err == nil {
			t.Fatalf("create body %q error = nil, want validation error", body)
		}
	}
	if _, err := store.CreateComment(ctx, comment.CreateInput{NoteID: commentStoreNoteID, UserID: commentStoreOwnerUserID, Body: strings.Repeat("😀", comment.BodyMaxLength)}); err != nil {
		t.Fatalf("create 1000-code-point body: %v", err)
	}

	for _, body := range []string{"", " ", strings.Repeat("a", comment.BodyMaxLength+1)} {
		if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), commentStoreNoteID, commentStoreOwnerUserID, body, 0); err == nil {
			t.Fatalf("direct body %q insert succeeded, want CHECK error", body)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), "missing-note", commentStoreOwnerUserID, "comentário", 0); err == nil {
		t.Fatal("direct missing note insert succeeded, want foreign key error")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), commentStoreNoteID, "missing-user", "comentário", 0); err == nil {
		t.Fatal("direct missing user insert succeeded, want foreign key error")
	}

	createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "apaga com a nota")
	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, commentStoreNoteID); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if got := countNoteComments(t, ctx, db, commentStoreNoteID); got != 0 {
		t.Fatalf("comments after note deletion = %d, want 0", got)
	}

	insertBareUsefulStoreUser(t, ctx, db, commentStoreBareUserID)
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), commentStoreOtherNoteID, commentStoreBareUserID, "apaga com o usuário", 0); err != nil {
		t.Fatalf("insert bare-user comment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, commentStoreBareUserID); err != nil {
		t.Fatalf("delete bare user: %v", err)
	}
	if got := countNoteComments(t, ctx, db, commentStoreOtherNoteID); got != 0 {
		t.Fatalf("comments after user deletion = %d, want 0", got)
	}
}

func TestCommentStoreTreatsBodiesAsDataAndKeepsCursorsMonotonic(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	for _, body := range []string{"' OR 1=1; --", `"; DROP TABLE notes; --`, "/* 👩‍💻 */"} {
		created := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, body)
		if created.Body != body {
			t.Fatalf("stored body = %q, want %q", created.Body, body)
		}
	}
	first := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "primeiro")
	second := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "segundo")
	third := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "terceiro")

	page, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list before delete: %v", err)
	}
	thirdPosition := positionForComment(t, page.Comments, third.ID)
	if err := store.DeleteComment(ctx, string(third.ID)); err != nil {
		t.Fatalf("delete maximum page key: %v", err)
	}
	replacement := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "reposição")

	all, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list after replacement: %v", err)
	}
	replacementPosition := positionForComment(t, all.Comments, replacement.ID)
	if replacementPosition.PageKey <= thirdPosition.PageKey {
		t.Fatalf("replacement page key = %d, want > %d", replacementPosition.PageKey, thirdPosition.PageKey)
	}
	continued, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10, After: &thirdPosition})
	if err != nil {
		t.Fatalf("list after deleted cursor: %v", err)
	}
	if diff := cmp.Diff([]comment.CommentID{replacement.ID}, commentIDs(continued.Comments)); diff != "" {
		t.Fatalf("continued ids mismatch (-want +got):\n%s", diff)
	}
	if first.ID == second.ID {
		t.Fatal("distinct comments share an id")
	}
}

func newCommentStoreTestStore(t *testing.T, ctx context.Context) (*CommentStore, *sql.DB, time.Time) {
	t.Helper()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 24, 12, 0, 0, 987654321, time.UTC)
	return newCommentStore(db, func() time.Time { return now }), db, now
}

func insertCommentStoreAuthor(t *testing.T, ctx context.Context, db execer, userID user.UserID, authorID author.AuthorID, displayName string) {
	t.Helper()
	insertAuthorStoreUser(t, ctx, db, userID, authorID, displayName)
}

func createComment(t *testing.T, ctx context.Context, store *CommentStore, noteID string, userID user.UserID, body string) comment.Comment {
	t.Helper()
	created, err := store.CreateComment(ctx, comment.CreateInput{NoteID: noteID, UserID: userID, Body: body})
	if err != nil {
		t.Fatalf("create comment %q: %v", body, err)
	}
	return created
}

func commentIDs(comments []comment.ListedComment) []comment.CommentID {
	ids := make([]comment.CommentID, 0, len(comments))
	for _, found := range comments {
		ids = append(ids, found.Comment.ID)
	}
	return ids
}

func positionForComment(t *testing.T, comments []comment.ListedComment, id comment.CommentID) comment.Position {
	t.Helper()
	for _, found := range comments {
		if found.Comment.ID == id {
			return found.Position
		}
	}
	t.Fatalf("comment %q not found", id)
	return comment.Position{}
}

func countNoteComments(t *testing.T, ctx context.Context, db *sql.DB, noteID string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_comments WHERE note_id = ?`, noteID).Scan(&count); err != nil {
		t.Fatalf("count note comments: %v", err)
	}
	return count
}

func createReply(t *testing.T, ctx context.Context, store *CommentStore, parentID comment.CommentID, userID user.UserID, body string) comment.Comment {
	t.Helper()
	created, err := store.CreateReply(ctx, comment.CreateReplyInput{ParentCommentID: parentID, UserID: userID, Body: body})
	if err != nil {
		t.Fatalf("create reply %q: %v", body, err)
	}
	return created
}

func replyIDs(replies []comment.Comment) []comment.CommentID {
	ids := make([]comment.CommentID, 0, len(replies))
	for _, reply := range replies {
		ids = append(ids, reply.ID)
	}
	return ids
}

func TestCommentStoreCreateReplyCarriesParentNoteID(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	parent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "comentário principal")
	reply := createReply(t, ctx, store, parent.ID, commentStoreOwnerUserID, "primeira resposta")

	if reply.NoteID != parent.NoteID {
		t.Fatalf("reply note id = %q, want parent note id %q", reply.NoteID, parent.NoteID)
	}
	if reply.ParentCommentID != parent.ID {
		t.Fatalf("reply parent id = %q, want %q", reply.ParentCommentID, parent.ID)
	}
	if reply.Body != "primeira resposta" {
		t.Fatalf("reply body = %q, want %q", reply.Body, "primeira resposta")
	}
}

func TestCommentStoreCreateReplyRejectsMissingParent(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	_, err := store.CreateReply(ctx, comment.CreateReplyInput{
		ParentCommentID: "missing-comment",
		UserID:          commentStoreOwnerUserID,
		Body:            "resposta",
	})
	if !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("create reply missing parent error = %v, want ErrCommentNotFound", err)
	}
}

func TestCommentStoreCreateReplyRejectsReplyParent(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	parent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "comentário principal")
	reply := createReply(t, ctx, store, parent.ID, commentStoreOwnerUserID, "primeira resposta")

	_, err := store.CreateReply(ctx, comment.CreateReplyInput{
		ParentCommentID: reply.ID,
		UserID:          commentStoreOwnerUserID,
		Body:            "resposta aninhada",
	})
	if !errors.Is(err, comment.ErrParentCommentNotTopLevel) {
		t.Fatalf("create reply to reply error = %v, want ErrParentCommentNotTopLevel", err)
	}
}

func TestCommentStoreDeleteCommentCascadesToReplies(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	parent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "comentário principal")
	firstReply := createReply(t, ctx, store, parent.ID, commentStoreOwnerUserID, "resposta um")
	createReply(t, ctx, store, parent.ID, commentStoreOwnerUserID, "resposta dois")

	if err := store.DeleteComment(ctx, string(parent.ID)); err != nil {
		t.Fatalf("delete parent comment: %v", err)
	}

	if _, err := store.FindCommentByID(ctx, string(firstReply.ID)); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("find deleted first reply error = %v, want ErrCommentNotFound", err)
	}
	if _, err := store.FindComment(ctx, commentStoreNoteID, string(firstReply.ID)); !errors.Is(err, comment.ErrCommentNotFound) {
		t.Fatalf("find deleted first reply by note error = %v, want ErrCommentNotFound", err)
	}
}

func TestCommentStoreListNoteCommentsNestsReplies(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	first := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "primeiro")
	second := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "segundo")
	firstReplyA := createReply(t, ctx, store, first.ID, commentStoreOwnerUserID, "resposta A")
	firstReplyB := createReply(t, ctx, store, first.ID, commentStoreOwnerUserID, "resposta B")

	page, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list note comments: %v", err)
	}
	if diff := cmp.Diff([]comment.CommentID{first.ID, second.ID}, commentIDs(page.Comments)); diff != "" {
		t.Fatalf("page ids mismatch (-want +got):\n%s", diff)
	}
	if len(page.Comments[0].Replies) != 2 {
		t.Fatalf("first comment replies = %d, want 2", len(page.Comments[0].Replies))
	}
	if diff := cmp.Diff([]comment.CommentID{firstReplyA.ID, firstReplyB.ID}, replyIDs(page.Comments[0].Replies)); diff != "" {
		t.Fatalf("first comment reply ids mismatch (-want +got):\n%s", diff)
	}
	if page.Comments[0].HasMoreReplies {
		t.Fatal("first comment HasMoreReplies = true, want false")
	}
	if len(page.Comments[1].Replies) != 0 {
		t.Fatalf("second comment replies = %d, want 0", len(page.Comments[1].Replies))
	}
	if page.Comments[1].HasMoreReplies {
		t.Fatal("second comment HasMoreReplies = true, want false")
	}
}

func TestCommentStoreListNoteCommentsCapsRepliesAtMaxPerParent(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	insertAuthorStoreNote(t, ctx, db, commentStoreOtherNoteID, commentStoreOwnerUserID, 0)

	overflowParent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "pai com estouro")
	for i := range comment.ReplyMaxPerParent + 3 {
		createReply(t, ctx, store, overflowParent.ID, commentStoreOwnerUserID, fmt.Sprintf("resposta %d", i))
	}

	exactParent := createComment(t, ctx, store, commentStoreOtherNoteID, commentStoreOwnerUserID, "pai exato")
	for i := range comment.ReplyMaxPerParent {
		createReply(t, ctx, store, exactParent.ID, commentStoreOwnerUserID, fmt.Sprintf("resposta %d", i))
	}

	overflowPage, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list overflow note comments: %v", err)
	}
	if len(overflowPage.Comments) != 1 {
		t.Fatalf("overflow page comments = %d, want 1", len(overflowPage.Comments))
	}
	if len(overflowPage.Comments[0].Replies) != comment.ReplyMaxPerParent {
		t.Fatalf("overflow replies = %d, want %d", len(overflowPage.Comments[0].Replies), comment.ReplyMaxPerParent)
	}
	if !overflowPage.Comments[0].HasMoreReplies {
		t.Fatal("overflow HasMoreReplies = false, want true")
	}

	exactPage, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreOtherNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list exact note comments: %v", err)
	}
	if len(exactPage.Comments) != 1 {
		t.Fatalf("exact page comments = %d, want 1", len(exactPage.Comments))
	}
	if len(exactPage.Comments[0].Replies) != comment.ReplyMaxPerParent {
		t.Fatalf("exact replies = %d, want %d", len(exactPage.Comments[0].Replies), comment.ReplyMaxPerParent)
	}
	if exactPage.Comments[0].HasMoreReplies {
		t.Fatal("exact HasMoreReplies = true, want false")
	}
}

func TestCommentStoreListNoteCommentsIsolatesRepliesByNote(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)
	insertAuthorStoreNote(t, ctx, db, commentStoreOtherNoteID, commentStoreOwnerUserID, 0)

	noteAParent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "nota A pai")
	createReply(t, ctx, store, noteAParent.ID, commentStoreOwnerUserID, "resposta na nota A")

	noteBParent := createComment(t, ctx, store, commentStoreOtherNoteID, commentStoreOwnerUserID, "nota B pai")
	noteBReply := createReply(t, ctx, store, noteBParent.ID, commentStoreOwnerUserID, "resposta isolada na nota B")

	page, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 10})
	if err != nil {
		t.Fatalf("list note A comments: %v", err)
	}
	for _, listed := range page.Comments {
		if listed.Comment.ID == noteBParent.ID {
			t.Fatalf("note B parent %q appeared in note A page", noteBParent.ID)
		}
		for _, reply := range listed.Replies {
			if reply.ID == noteBReply.ID || strings.Contains(reply.Body, "nota B") {
				t.Fatalf("note B reply %q leaked into note A page under parent %q", reply.ID, listed.Comment.ID)
			}
		}
	}
}

func TestCommentStoreListNoteCommentsPaginatesTopLevelIndependentlyOfReplies(t *testing.T) {
	ctx := context.Background()
	store, db, _ := newCommentStoreTestStore(t, ctx)
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	first := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "primeiro")
	second := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "segundo")
	third := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, "terceiro")
	createReply(t, ctx, store, first.ID, commentStoreOwnerUserID, "resposta no primeiro")
	createReply(t, ctx, store, second.ID, commentStoreOwnerUserID, "resposta no segundo")

	firstPage, err := store.ListNoteComments(ctx, comment.ListInput{NoteID: commentStoreNoteID, Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if !firstPage.HasMore {
		t.Fatal("first page HasMore = false, want true")
	}
	if diff := cmp.Diff([]comment.CommentID{first.ID, second.ID}, commentIDs(firstPage.Comments)); diff != "" {
		t.Fatalf("first page ids mismatch (-want +got):\n%s", diff)
	}

	secondPage, err := store.ListNoteComments(ctx, comment.ListInput{
		NoteID: commentStoreNoteID,
		Limit:  2,
		After:  &firstPage.Comments[1].Position,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if secondPage.HasMore {
		t.Fatal("second page HasMore = true, want false")
	}
	if diff := cmp.Diff([]comment.CommentID{third.ID}, commentIDs(secondPage.Comments)); diff != "" {
		t.Fatalf("second page ids mismatch (-want +got):\n%s", diff)
	}
}

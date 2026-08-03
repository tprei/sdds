// Comment-store tests covering one-level reply creation, cascade delete, and
// the nesting, capping, isolation, and top-level pagination of replies.
package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/user"
)

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

//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestAuthDeleteUserRuntimeBoundaries proves the account-deletion boundary
// against the live Compose API: a wrong password is rejected with the account
// intact, and a correct password removes the account, its author profile, its
// notes, and the comments it left on someone else's note.
func TestAuthDeleteUserRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	usernameA := fmt.Sprintf("delete-a-%d", time.Now().UnixNano())
	usernameB := fmt.Sprintf("delete-b-%d", time.Now().UnixNano())
	password := "secret-password"

	sessionA := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    usernameA,
		Password:    password,
		DisplayName: "Usuário A",
	})
	sessionB := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    usernameB,
		Password:    password,
		DisplayName: "Usuário B",
	})
	clientA := newAuthenticatedAPIClient(t, sessionA.Token)
	clientB := newAuthenticatedAPIClient(t, sessionB.Token)

	noteA := createNote(t, clientA, openapi.CreateNoteJSONRequestBody{
		Title:           "Nota que vai sumir",
		Body:            "corpo da nota para exclusão de conta",
		CategorySlug:    "food",
		ClientRequestId: "delete-account-note-a",
	})
	noteB := createNote(t, clientB, openapi.CreateNoteJSONRequestBody{
		Title:           "Nota do outro usuário",
		Body:            "outra nota que permanece",
		CategorySlug:    "food",
		ClientRequestId: "delete-account-note-b",
	})

	commentCtx := context.Background()
	comment, err := clientA.CreateNoteCommentWithResponse(commentCtx, noteB.Id, openapi.CreateNoteCommentJSONRequestBody{Body: "comentário que some junto com a conta"})
	if err != nil {
		t.Fatalf("POST comment as user A: %v", err)
	}
	if comment.StatusCode() != http.StatusCreated {
		t.Fatalf("POST comment status = %d, want %d", comment.StatusCode(), http.StatusCreated)
	}

	// A wrong password is rejected and the account stays usable.
	wrongPassword, err := clientA.DeleteAuthUserWithResponse(context.Background(), openapi.DeleteAuthUserJSONRequestBody{Password: "wrong-password"})
	if err != nil {
		t.Fatalf("DELETE account with wrong password: %v", err)
	}
	requireStatus(t, "DELETE account wrong password", wrongPassword.StatusCode(), http.StatusForbidden, wrongPassword.Body)
	if wrongPassword.JSON403 == nil || wrongPassword.JSON403.Code != openapi.ErrorCodeForbidden {
		t.Fatalf("DELETE account wrong password body = %#v, want forbidden", wrongPassword.JSON403)
	}
	requireCurrentSession(t, getAuthSession(t, clientA), sessionA)

	// The correct password deletes the account permanently.
	deleted, err := clientA.DeleteAuthUserWithResponse(context.Background(), openapi.DeleteAuthUserJSONRequestBody{Password: password})
	if err != nil {
		t.Fatalf("DELETE account: %v", err)
	}
	requireStatus(t, "DELETE account", deleted.StatusCode(), http.StatusNoContent, deleted.Body)

	// The deleted account can no longer authenticate.
	requireInvalidAuthSession(t, publicClient, usernameA, password)
	// The old bearer is dead.
	requireUnauthenticatedAuthSession(t, clientA)
	// The author profile is gone.
	authorA := sessionA.User.Author.Id
	profile, err := clientB.GetAuthorWithResponse(context.Background(), authorA)
	if err != nil {
		t.Fatalf("GET author A after delete: %v", err)
	}
	requireStatus(t, "GET author A after delete", profile.StatusCode(), http.StatusNotFound, profile.Body)
	// The note is absent from the feed and from search.
	requireNoteNotListed(t, listNotes(t, clientB), noteA.Id)
	for _, result := range searchNotes(t, clientB, "exclusão de conta").Results {
		if result.Note.Id == noteA.Id {
			t.Fatalf("deleted user note %s still found by search", noteA.Id)
		}
	}
	// The comment the deleted user left on user B's note is gone from the thread.
	comments, err := clientB.ListNoteCommentsWithResponse(context.Background(), noteB.Id, nil)
	if err != nil {
		t.Fatalf("GET comments on note B: %v", err)
	}
	for _, thread := range comments.JSON200.Threads {
		if thread.Comment.Id == comment.JSON201.Id {
			t.Fatalf("comment %s from deleted user still present on note B", thread.Comment.Id)
		}
		for _, reply := range thread.Replies {
			if reply.Id == comment.JSON201.Id {
				t.Fatalf("reply %s from deleted user still present on note B", reply.Id)
			}
		}
	}
	// User B is untouched.
	requireCurrentSession(t, getAuthSession(t, clientB), sessionB)
}

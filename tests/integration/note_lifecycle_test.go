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

// TestNoteLifecycleRuntimeBoundaries proves the Compose-to-runtime note
// lifecycle boundary: create, list, category filter, and detail fetch against
// the assembled image, migrations, router, and SQLite persistence. It asserts
// membership of its own notes rather than an absolute count so it stays
// independent of other tests sharing the stack.
func TestNoteLifecycleRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("notes-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Notas Runtime",
	})
	client := newAuthenticatedAPIClient(t, session.Token)

	request := openapi.CreateNoteJSONRequestBody{
		Title:           "Café bom",
		Body:            "Tem pao de queijo decente e balcao simpatico.",
		CategorySlug:    "food",
		ClientRequestId: "integration-created-note",
	}
	created := createNote(t, client, request)
	requireCreatedNote(t, created, request)

	travelRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Dica de viagem",
		Body:            "Serve para qualquer lugar mundial.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-travel-note",
	}
	travelNote := createNote(t, client, travelRequest)
	requireCreatedNote(t, travelNote, travelRequest)

	updatedNotes := listNotes(t, client)
	requireListedNote(t, updatedNotes, created.Id, request)
	requireListedNote(t, updatedNotes, travelNote.Id, travelRequest)

	foodNotes := listNotesByCategory(t, client, "food")
	requireListedNote(t, foodNotes, created.Id, request)
	requireNoteNotListed(t, foodNotes, travelNote.Id)

	travelNotes := listNotesByCategory(t, client, "travel")
	requireListedNote(t, travelNotes, travelNote.Id, travelRequest)
	requireNoteNotListed(t, travelNotes, created.Id)

	fetched := getNote(t, client, created.Id)
	requireCreatedNote(t, fetched, request)
	if fetched.Id != created.Id {
		t.Fatalf("fetched note id = %q, want %q", fetched.Id, created.Id)
	}
	if fetched.CreatedAt != created.CreatedAt {
		t.Fatalf("fetched created_at = %d, want %d", fetched.CreatedAt, created.CreatedAt)
	}
	if fetched.UpdatedAt != created.UpdatedAt {
		t.Fatalf("fetched updated_at = %d, want %d", fetched.UpdatedAt, created.UpdatedAt)
	}

	fetchedTravelNote := getNote(t, client, travelNote.Id)
	requireCreatedNote(t, fetchedTravelNote, travelRequest)
	if fetchedTravelNote.Id != travelNote.Id {
		t.Fatalf("fetched travel note id = %q, want %q", fetchedTravelNote.Id, travelNote.Id)
	}

	requireListNotesCategoryFilterError(t, client, "comida")
}

// TestNoteEditDeleteRuntimeBoundaries proves the author edit and delete paths
// against the assembled image: a PATCH re-indexes lexical and semantic search,
// recategorizes the note, and bumps updated_at; a non-author is forbidden;
// a DELETE removes the note from every surface and cascades its comments.
func TestNoteEditDeleteRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("edit-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Editor Runtime",
	})
	authorA := newAuthenticatedAPIClient(t, session.Token)
	authorAID := session.User.Author.Id

	otherSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("other-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Outra Runtime",
	})
	authorB := newAuthenticatedAPIClient(t, otherSession.Token)

	originalToken := "zorglub-magnifico"
	note := createNote(t, authorA, openapi.CreateNoteJSONRequestBody{
		Title:           "Achado original " + originalToken,
		Body:            "Texto com o token " + originalToken + " pra busca lexical.",
		CategorySlug:    "food",
		ClientRequestId: "edit-original",
	})

	// The note is found by its original token before any edit.
	requireSearchResultByID(t, searchNotes(t, authorA, originalToken), note.Id)

	// A non-author PATCH is forbidden.
	forbiddenPatch, err := authorB.UpdateNoteWithResponse(context.Background(), note.Id, openapi.UpdateNoteJSONRequestBody{Body: ptr("tentativa alheia")})
	if err != nil {
		t.Fatalf("PATCH as non-author: %v", err)
	}
	requireForbidden(t, "PATCH as non-author", forbiddenPatch.StatusCode(), forbiddenPatch.Body)

	// A PATCH of an unknown note is not found.
	missingPatch, err := authorA.UpdateNoteWithResponse(context.Background(), "never-existed", openapi.UpdateNoteJSONRequestBody{Body: ptr("nada")})
	if err != nil {
		t.Fatalf("PATCH unknown note: %v", err)
	}
	requireStatus(t, "PATCH unknown note", missingPatch.StatusCode(), http.StatusNotFound, missingPatch.Body)

	// The author edits the body to a new token; updated_at advances.
	editedToken := "esparadrapo-reluzente"
	updated := updateNote(t, authorA, note.Id, openapi.UpdateNoteJSONRequestBody{
		Title: ptr("Achado editado " + editedToken),
		Body:  ptr("Agora o token é " + editedToken + "."),
	})
	if updated.UpdatedAt <= updated.CreatedAt {
		t.Fatalf("updated_at = %d, created_at = %d, want updated_at > created_at", updated.UpdatedAt, updated.CreatedAt)
	}

	// Lexical search reflects the edit: the new token matches, the old one does not.
	requireSearchResultByID(t, searchNotes(t, authorA, editedToken), note.Id)
	requireNeverLexicallyMatched(t, searchNotes(t, authorA, originalToken), note.Id)

	// Recategorize from food to travel; category listings follow.
	updateNote(t, authorA, note.Id, openapi.UpdateNoteJSONRequestBody{CategorySlug: ptr(openapi.CategorySlug("travel"))})
	requireListedNote(t, listNotesByCategory(t, authorA, "travel"), note.Id, openapi.CreateNoteJSONRequestBody{
		Title: "Achado editado " + editedToken, Body: "Agora o token é " + editedToken + ".", CategorySlug: "travel", ClientRequestId: "edit-original",
	})
	requireNoteNotListed(t, listNotesByCategory(t, authorA, "food"), note.Id)

	// A non-author DELETE is forbidden; an unknown-id DELETE is not found.
	forbiddenDelete, err := authorB.DeleteNoteWithResponse(context.Background(), note.Id)
	if err != nil {
		t.Fatalf("DELETE as non-author: %v", err)
	}
	requireForbidden(t, "DELETE as non-author", forbiddenDelete.StatusCode(), forbiddenDelete.Body)
	missingDelete, err := authorA.DeleteNoteWithResponse(context.Background(), "never-existed")
	if err != nil {
		t.Fatalf("DELETE unknown note: %v", err)
	}
	requireStatus(t, "DELETE unknown note", missingDelete.StatusCode(), http.StatusNotFound, missingDelete.Body)

	// Add a comment, then delete the note: the comment and the note vanish from every surface.
	if _, err := authorA.CreateNoteCommentWithResponse(context.Background(), note.Id, openapi.CreateNoteCommentJSONRequestBody{Body: "comentário que some junto"}); err != nil {
		t.Fatalf("POST comment: %v", err)
	}

	deleteNote(t, authorA, note.Id)
	requireNoteNotFound(t, authorA, note.Id)
	requireNoteNotListed(t, listNotes(t, authorA), note.Id)
	authorPage := listAuthorNotes(t, authorA, authorAID, &openapi.ListAuthorNotesParams{Limit: ptr(50)})
	requireNoteNotListed(t, openapi.ListNotesResponse{Notes: authorPage.Notes}, note.Id)
	for _, result := range searchNotes(t, authorA, editedToken).Results {
		if result.Note.Id == note.Id {
			t.Fatalf("deleted note still found by search token %q", editedToken)
		}
	}
}

func ptr[T any](value T) *T { return &value }

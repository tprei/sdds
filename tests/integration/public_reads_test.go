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

// TestPublicReadsRuntimeBoundaries proves the anonymous read surface against the
// assembled stack: a signed-out client reads a note, its author, the author's
// notes, the note's comments, the feed, the categories, and a search, all
// without a token. Viewer-only fields (useful_by_current_user) must be absent
// from every note the anonymous client observes.
func TestPublicReadsRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("public-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Leitor Público",
	})
	client := newAuthenticatedAPIClient(t, session.Token)

	created := createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:           "Café bom",
		Body:            "Tem pao de queijo decente e balcao simpatico.",
		CategorySlug:    "food",
		ClientRequestId: "integration-public-read",
	})
	authorID := created.Author.Id

	// The author has marked the note useful, so an authenticated read would
	// carry useful_by_current_user=true. The anonymous read must omit it.
	markResponse, err := client.MarkNoteUsefulWithResponse(context.Background(), created.Id)
	if err != nil {
		t.Fatalf("PUT /v1/notes/{id}/useful: %v", err)
	}
	requireStatus(t, "PUT useful", markResponse.StatusCode(), http.StatusNoContent, markResponse.Body)

	t.Run("note omits viewer field", func(t *testing.T) {
		note := getNote(t, publicClient, created.Id)
		if note.UsefulByCurrentUser != nil {
			t.Fatalf("anonymous note read includes useful_by_current_user = %v", *note.UsefulByCurrentUser)
		}
	})

	t.Run("feed omits viewer field", func(t *testing.T) {
		for _, note := range listNotes(t, publicClient).Notes {
			if note.Id == created.Id && note.UsefulByCurrentUser != nil {
				t.Fatalf("anonymous feed includes useful_by_current_user for %s", note.Id)
			}
		}
	})

	t.Run("search omits viewer field", func(t *testing.T) {
		results := searchNotes(t, publicClient, "café")
		for _, result := range results.Results {
			if result.Note.Id == created.Id && result.Note.UsefulByCurrentUser != nil {
				t.Fatalf("anonymous search includes useful_by_current_user for %s", result.Note.Id)
			}
		}
	})

	t.Run("author profile is public", func(t *testing.T) {
		response, err := publicClient.GetAuthorWithResponse(context.Background(), authorID)
		if err != nil {
			t.Fatalf("GET /v1/authors/{id}: %v", err)
		}
		requireStatus(t, "GET author", response.StatusCode(), http.StatusOK, response.Body)
		if response.JSON200 == nil || response.JSON200.Id != authorID {
			t.Fatalf("GET author body = %+v", response.JSON200)
		}
	})

	t.Run("author notes omit viewer field", func(t *testing.T) {
		response, err := publicClient.ListAuthorNotesWithResponse(context.Background(), authorID, nil)
		if err != nil {
			t.Fatalf("GET /v1/authors/{id}/notes: %v", err)
		}
		requireStatus(t, "GET author notes", response.StatusCode(), http.StatusOK, response.Body)
		if response.JSON200 == nil {
			t.Fatal("GET author notes returned 200 without JSON body")
		}
		for _, note := range response.JSON200.Notes {
			if note.Id == created.Id && note.UsefulByCurrentUser != nil {
				t.Fatalf("anonymous author notes include useful_by_current_user for %s", note.Id)
			}
		}
	})

	t.Run("note comments are public", func(t *testing.T) {
		response, err := publicClient.ListNoteCommentsWithResponse(context.Background(), created.Id, nil)
		if err != nil {
			t.Fatalf("GET /v1/notes/{id}/comments: %v", err)
		}
		requireStatus(t, "GET note comments", response.StatusCode(), http.StatusOK, response.Body)
	})

	t.Run("categories are public", func(t *testing.T) {
		requireCatalogs(t, publicClient)
	})
}

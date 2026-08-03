//go:build integration

package integration

import (
	"fmt"
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

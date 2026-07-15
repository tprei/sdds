//go:build integration

package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/openapi"
)

func TestPublicAuthorProfileRuntimeBoundaries(t *testing.T) {
	client := newAPIClient(t)
	waitForReadiness(t, client)

	suffix := time.Now().UnixNano()
	firstDisplayName := "Autora Integração"
	secondDisplayName := "Outro Autor"
	firstSession := createAuthUser(t, client, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("author-a-%d", suffix),
		Password:    "secret-password",
		DisplayName: firstDisplayName,
	})
	secondSession := createAuthUser(t, client, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("author-b-%d", suffix),
		Password:    "secret-password",
		DisplayName: secondDisplayName,
	})

	firstClient := newAuthenticatedAPIClient(t, firstSession.Token)
	secondClient := newAuthenticatedAPIClient(t, secondSession.Token)
	createdFirstNotes := []openapi.Note{
		createNote(t, firstClient, openapi.CreateNoteJSONRequestBody{Title: fmt.Sprintf("Perfil A 1 %d", suffix), Body: "Primeira nota da autora.", CategorySlug: "food", ClientRequestId: fmt.Sprintf("author-a-1-%d", suffix)}),
		createNote(t, firstClient, openapi.CreateNoteJSONRequestBody{Title: fmt.Sprintf("Perfil A 2 %d", suffix), Body: "Segunda nota da autora.", CategorySlug: "travel", ClientRequestId: fmt.Sprintf("author-a-2-%d", suffix)}),
		createNote(t, firstClient, openapi.CreateNoteJSONRequestBody{Title: fmt.Sprintf("Perfil A 3 %d", suffix), Body: "Terceira nota da autora.", CategorySlug: "finds", ClientRequestId: fmt.Sprintf("author-a-3-%d", suffix)}),
	}
	secondNote := createNote(t, secondClient, openapi.CreateNoteJSONRequestBody{Title: fmt.Sprintf("Perfil B %d", suffix), Body: "Nota do segundo autor.", CategorySlug: "food", ClientRequestId: fmt.Sprintf("author-b-1-%d", suffix)})

	publicClient := newAPIClient(t)
	profileResponse, err := publicClient.GetAuthorWithResponse(context.Background(), firstSession.User.Author.Id)
	if err != nil {
		t.Fatalf("GET /v1/authors/{author_id}: %v", err)
	}
	requireStatus(t, "GET /v1/authors/{author_id}", profileResponse.StatusCode(), http.StatusOK, profileResponse.Body)
	if profileResponse.JSON200 == nil {
		t.Fatal("GET /v1/authors/{author_id} returned 200 without JSON body")
	}
	wantProfile := openapi.PublicAuthor{Id: firstSession.User.Author.Id, DisplayName: firstDisplayName, NoteCount: int64(len(createdFirstNotes))}
	if diff := cmp.Diff(wantProfile, *profileResponse.JSON200); diff != "" {
		t.Fatalf("public author mismatch (-want +got):\n%s", diff)
	}
	requirePublicAuthorWireKeys(t, profileResponse.Body)
	requireNoPrivateAuthorPayload(t, profileResponse.Body)

	limit := 2
	firstPage, firstPageBody := listAuthorNotesWithBody(t, publicClient, firstSession.User.Author.Id, &openapi.ListAuthorNotesParams{Limit: &limit})
	if len(firstPage.Notes) != limit {
		t.Fatalf("first author page count = %d, want %d", len(firstPage.Notes), limit)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page next_cursor = nil, want cursor")
	}

	secondPage, secondPageBody := listAuthorNotesWithBody(t, publicClient, firstSession.User.Author.Id, &openapi.ListAuthorNotesParams{Limit: &limit, Cursor: firstPage.NextCursor})
	if secondPage.NextCursor != nil {
		t.Fatalf("second page next_cursor = %q, want nil", *secondPage.NextCursor)
	}
	allFirstAuthorNotes := append([]openapi.Note{}, firstPage.Notes...)
	allFirstAuthorNotes = append(allFirstAuthorNotes, secondPage.Notes...)
	requireAuthorNotesMatchCreated(t, allFirstAuthorNotes, createdFirstNotes, firstSession.User.Author, secondNote.Id)
	requireAuthorNotesWirePayload(t, firstPageBody)
	requireAuthorNotesWirePayload(t, secondPageBody)

	secondProfileResponse, err := publicClient.GetAuthorWithResponse(context.Background(), secondSession.User.Author.Id)
	if err != nil {
		t.Fatalf("GET /v1/authors/{second_author_id}: %v", err)
	}
	requireStatus(t, "GET /v1/authors/{second_author_id}", secondProfileResponse.StatusCode(), http.StatusOK, secondProfileResponse.Body)
	if secondProfileResponse.JSON200 == nil {
		t.Fatal("GET /v1/authors/{second_author_id} returned 200 without JSON body")
	}
	if secondProfileResponse.JSON200.NoteCount != 1 {
		t.Fatalf("second profile note_count = %d, want 1", secondProfileResponse.JSON200.NoteCount)
	}
	secondNotes := listAuthorNotes(t, publicClient, secondSession.User.Author.Id, &openapi.ListAuthorNotesParams{Limit: &limit})
	if len(secondNotes.Notes) != 1 || secondNotes.Notes[0].Id != secondNote.Id {
		t.Fatalf("second author notes = %#v, want only %q", secondNotes.Notes, secondNote.Id)
	}

	requireUnknownAuthorErrors(t, publicClient)
	requireInvalidAuthorNotesParams(t, publicClient, firstSession.User.Author.Id)
}

func listAuthorNotes(t *testing.T, client *openapi.ClientWithResponses, authorID string, params *openapi.ListAuthorNotesParams) openapi.AuthorNotesPage {
	t.Helper()
	page, _ := listAuthorNotesWithBody(t, client, authorID, params)
	return page
}

func listAuthorNotesWithBody(t *testing.T, client *openapi.ClientWithResponses, authorID string, params *openapi.ListAuthorNotesParams) (openapi.AuthorNotesPage, []byte) {
	t.Helper()
	response, err := client.ListAuthorNotesWithResponse(context.Background(), authorID, params)
	if err != nil {
		t.Fatalf("GET /v1/authors/{author_id}/notes: %v", err)
	}
	requireStatus(t, "GET /v1/authors/{author_id}/notes", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/authors/{author_id}/notes returned 200 without JSON body")
	}
	return *response.JSON200, response.Body
}

func requireAuthorNotesMatchCreated(t *testing.T, got []openapi.Note, created []openapi.Note, author openapi.AuthorSummary, forbiddenNoteID string) {
	t.Helper()
	if len(got) != len(created) {
		t.Fatalf("author note count = %d, want %d", len(got), len(created))
	}
	seen := make(map[string]bool, len(got))
	for index, found := range got {
		if index > 0 && !noteBeforeOrSame(got[index-1], found) {
			t.Fatalf("notes are not ordered by created_at DESC, id DESC: %#v", got)
		}
		if found.Id == forbiddenNoteID {
			t.Fatalf("author notes included second author note %q", forbiddenNoteID)
		}
		if found.Author != author {
			t.Fatalf("note author = %#v, want %#v", found.Author, author)
		}
		if seen[found.Id] {
			t.Fatalf("duplicate note id %q", found.Id)
		}
		seen[found.Id] = true
	}
	for _, createdNote := range created {
		if !seen[createdNote.Id] {
			t.Fatalf("created note %q missing from author page union", createdNote.Id)
		}
	}
}

func noteBeforeOrSame(left openapi.Note, right openapi.Note) bool {
	if left.CreatedAt != right.CreatedAt {
		return left.CreatedAt > right.CreatedAt
	}
	return left.Id > right.Id
}

func requireUnknownAuthorErrors(t *testing.T, client *openapi.ClientWithResponses) {
	t.Helper()
	unknownAuthorID := "018ff5b8-0000-7000-8000-000000099999"
	profile, err := client.GetAuthorWithResponse(context.Background(), unknownAuthorID)
	if err != nil {
		t.Fatalf("GET /v1/authors/{unknown}: %v", err)
	}
	requireStatus(t, "GET /v1/authors/{unknown}", profile.StatusCode(), http.StatusNotFound, profile.Body)
	if profile.JSON404 == nil || profile.JSON404.Code != openapi.ErrorCodeNotFound {
		t.Fatalf("unknown author profile body = %#v, want not_found", profile.JSON404)
	}
	notes, err := client.ListAuthorNotesWithResponse(context.Background(), unknownAuthorID, nil)
	if err != nil {
		t.Fatalf("GET /v1/authors/{unknown}/notes: %v", err)
	}
	requireStatus(t, "GET /v1/authors/{unknown}/notes", notes.StatusCode(), http.StatusNotFound, notes.Body)
	if notes.JSON404 == nil || notes.JSON404.Code != openapi.ErrorCodeNotFound {
		t.Fatalf("unknown author notes body = %#v, want not_found", notes.JSON404)
	}
}

func requireInvalidAuthorNotesParams(t *testing.T, client *openapi.ClientWithResponses, authorID string) {
	t.Helper()
	zero := 0
	overMax := 51
	malformed := "not-base64!"
	unsupported := base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"created_at":1782993600000,"id":"018ff5b8-0000-7000-8000-000000000012"}`))
	tests := []struct {
		name      string
		params    *openapi.ListAuthorNotesParams
		wantField openapi.ValidationField
	}{
		{name: "zero limit", params: &openapi.ListAuthorNotesParams{Limit: &zero}, wantField: openapi.ValidationFieldLimit},
		{name: "over max limit", params: &openapi.ListAuthorNotesParams{Limit: &overMax}, wantField: openapi.ValidationFieldLimit},
		{name: "malformed cursor", params: &openapi.ListAuthorNotesParams{Cursor: &malformed}, wantField: openapi.ValidationFieldCursor},
		{name: "unsupported cursor", params: &openapi.ListAuthorNotesParams{Cursor: &unsupported}, wantField: openapi.ValidationFieldCursor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.ListAuthorNotesWithResponse(context.Background(), authorID, tt.params)
			if err != nil {
				t.Fatalf("GET /v1/authors/{author_id}/notes invalid: %v", err)
			}
			requireStatus(t, "GET /v1/authors/{author_id}/notes invalid", response.StatusCode(), http.StatusBadRequest, response.Body)
			if response.JSON400 == nil {
				t.Fatal("invalid author notes returned 400 without JSON body")
			}
			requireValidationField(t, *response.JSON400, tt.wantField)
		})
	}
}

func requireValidationField(t *testing.T, got openapi.ErrorResponse, field openapi.ValidationField) {
	t.Helper()
	if got.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", got.Code, openapi.ErrorCodeInvalidNote)
	}
	if got.Fields == nil {
		t.Fatalf("fields = nil, want %s invalid", field)
	}
	want := []openapi.ValidationProblem{{Field: field, Code: openapi.ValidationProblemCodeInvalid}}
	if diff := cmp.Diff(want, *got.Fields); diff != "" {
		t.Fatalf("validation fields mismatch (-want +got):\n%s", diff)
	}
}

func requireAuthorNotesWirePayload(t *testing.T, body []byte) {
	t.Helper()
	requireNoPrivateAuthorPayload(t, body)
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatalf("decode author notes wire body: %v", err)
	}
	requireWireKeys(t, object, "notes", "next_cursor")
	notes, ok := object["notes"].([]any)
	if !ok {
		t.Fatalf("notes = %T, want array", object["notes"])
	}
	for _, value := range notes {
		noteObject, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("note = %T, want object", value)
		}
		requireWireKeys(t, noteObject, "id", "title", "body", "category_slug", "place_slug", "author", "images", "created_at", "updated_at")
		images, ok := noteObject["images"].([]any)
		if !ok || len(images) != 0 {
			t.Fatalf("note images = %#v, want empty array", noteObject["images"])
		}
		authorObject, ok := noteObject["author"].(map[string]any)
		if !ok {
			t.Fatalf("author = %T, want object", noteObject["author"])
		}
		requireWireKeys(t, authorObject, "id", "display_name")
	}
}

func requirePublicAuthorWireKeys(t *testing.T, body []byte) {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatalf("decode public author wire body: %v", err)
	}
	requireWireKeys(t, object, "id", "display_name", "note_count")
}

func requireWireKeys(t *testing.T, object map[string]any, keys ...string) {
	t.Helper()
	want := make(map[string]bool, len(keys))
	for _, key := range keys {
		want[key] = true
	}
	for key := range want {
		if _, ok := object[key]; !ok {
			t.Fatalf("missing JSON key %q in %#v", key, object)
		}
	}
	for key := range object {
		if !want[key] {
			t.Fatalf("unexpected JSON key %q in %#v", key, object)
		}
	}
}

func requireNoPrivateAuthorPayload(t *testing.T, body []byte) {
	t.Helper()
	privateFragments := []string{"user_id", "username", "account_state", "login_identity", "secret_hash", "password", "token", "session_id", "session_expiry", "expires_at"}
	payload := string(body)
	for _, fragment := range privateFragments {
		if strings.Contains(payload, fragment) {
			t.Fatalf("payload contains private fragment %q: %s", fragment, payload)
		}
	}
}

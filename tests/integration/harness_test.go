//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/openapi"
)

const (
	defaultAPIBaseURL = "http://127.0.0.1:8080"
	httpClientTimeout = 5 * time.Second
	readyTimeout      = 30 * time.Second
)

type noteFields struct {
	Title        string
	Body         string
	CategorySlug string
}

func requireCatalogs(t *testing.T, client *openapi.ClientWithResponses) {
	t.Helper()

	categories, err := client.ListCategoriesWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/categories: %v", err)
	}
	requireStatus(t, "GET /v1/categories", categories.StatusCode(), http.StatusOK, categories.Body)
	if categories.JSON200 == nil {
		t.Fatal("GET /v1/categories returned 200 without JSON body")
	}
	wantCategories := []openapi.CatalogCategory{
		{Active: true, DisplayOrder: 10, Label: "Beleza", Slug: "beauty"},
		{Active: true, DisplayOrder: 20, Label: "Comida", Slug: "food"},
		{Active: true, DisplayOrder: 30, Label: "Viagem", Slug: "travel"},
		{Active: true, DisplayOrder: 40, Label: "Achadinhos", Slug: "finds"},
	}
	if diff := cmp.Diff(wantCategories, categories.JSON200.Categories); diff != "" {
		t.Fatalf("categories mismatch (-want +got):\n%s", diff)
	}
}

func newAPIClient(t *testing.T) *openapi.ClientWithResponses {
	t.Helper()

	client, err := openapi.NewClientWithResponses(
		apiBaseURL(),
		openapi.WithHTTPClient(&http.Client{Timeout: httpClientTimeout}),
	)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	return client
}

func newAuthenticatedAPIClient(t *testing.T, token string) *openapi.ClientWithResponses {
	t.Helper()

	client, err := openapi.NewClientWithResponses(
		apiBaseURL(),
		openapi.WithHTTPClient(&http.Client{Timeout: httpClientTimeout}),
		openapi.WithRequestEditorFn(bearerTokenEditor(token)),
	)
	if err != nil {
		t.Fatalf("create authenticated API client: %v", err)
	}
	return client
}

func apiBaseURL() string {
	if value := os.Getenv("SDDS_API_BASE_URL"); value != "" {
		return value
	}
	return defaultAPIBaseURL
}

func waitForReadiness(t *testing.T, client *openapi.ClientWithResponses) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), readyTimeout)
	defer cancel()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		response, err := client.GetReadinessWithResponse(ctx)
		if err == nil {
			if response.StatusCode() == http.StatusNoContent {
				return
			}
			lastErr = fmt.Errorf("status %d body %s", response.StatusCode(), string(response.Body))
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			t.Fatalf("api was not ready within %s: %v", readyTimeout, lastErr)
		case <-ticker.C:
		}
	}
}

func listNotes(t *testing.T, client *openapi.ClientWithResponses) openapi.ListNotesResponse {
	t.Helper()

	response, err := client.ListNotesWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("GET /v1/notes: %v", err)
	}
	requireStatus(t, "GET /v1/notes", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/notes returned 200 without JSON body")
	}
	return *response.JSON200
}

func listNotesByCategory(t *testing.T, client *openapi.ClientWithResponses, category string) openapi.ListNotesResponse {
	t.Helper()

	categorySlug := openapi.CategorySlug(category)
	response, err := client.ListNotesWithResponse(context.Background(), &openapi.ListNotesParams{CategorySlug: &categorySlug})
	if err != nil {
		t.Fatalf("GET /v1/notes?category_slug=%s: %v", category, err)
	}
	requireStatus(t, "GET /v1/notes?category_slug", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/notes?category_slug returned 200 without JSON body")
	}
	return *response.JSON200
}

func createNote(t *testing.T, client *openapi.ClientWithResponses, request openapi.CreateNoteJSONRequestBody) openapi.Note {
	t.Helper()

	response, err := client.CreateNoteWithResponse(context.Background(), request)
	if err != nil {
		t.Fatalf("POST /v1/notes: %v", err)
	}
	requireStatus(t, "POST /v1/notes", response.StatusCode(), http.StatusCreated, response.Body)
	if response.JSON201 == nil {
		t.Fatal("POST /v1/notes returned 201 without JSON body")
	}
	return *response.JSON201
}

func createAuthUser(t *testing.T, client *openapi.ClientWithResponses, request openapi.CreateAuthUserJSONRequestBody) openapi.AuthSessionResponse {
	t.Helper()

	response, err := client.CreateAuthUserWithResponse(context.Background(), request)
	if err != nil {
		t.Fatalf("POST /v1/auth/users: %v", err)
	}
	requireStatus(t, "POST /v1/auth/users", response.StatusCode(), http.StatusCreated, response.Body)
	if response.JSON201 == nil {
		t.Fatal("POST /v1/auth/users returned 201 without JSON body")
	}
	return *response.JSON201
}

func requireDuplicateAuthUser(t *testing.T, client *openapi.ClientWithResponses, request openapi.CreateAuthUserJSONRequestBody) {
	t.Helper()

	response, err := client.CreateAuthUserWithResponse(context.Background(), request)
	if err != nil {
		t.Fatalf("POST /v1/auth/users duplicate: %v", err)
	}
	requireStatus(t, "POST /v1/auth/users duplicate", response.StatusCode(), http.StatusConflict, response.Body)
	if response.JSON409 == nil {
		t.Fatal("POST /v1/auth/users duplicate returned 409 without JSON body")
	}
	if response.JSON409.Code != openapi.ErrorCodeUsernameTaken {
		t.Fatalf("duplicate code = %s, want %s", response.JSON409.Code, openapi.ErrorCodeUsernameTaken)
	}
}

func createAuthSession(t *testing.T, client *openapi.ClientWithResponses, request openapi.CreateAuthSessionJSONRequestBody) openapi.AuthSessionResponse {
	t.Helper()

	response, err := client.CreateAuthSessionWithResponse(context.Background(), request)
	if err != nil {
		t.Fatalf("POST /v1/auth/sessions: %v", err)
	}
	requireStatus(t, "POST /v1/auth/sessions", response.StatusCode(), http.StatusCreated, response.Body)
	if response.JSON201 == nil {
		t.Fatal("POST /v1/auth/sessions returned 201 without JSON body")
	}
	return *response.JSON201
}

func requireInvalidAuthSession(t *testing.T, client *openapi.ClientWithResponses, username string, password string) {
	t.Helper()

	response, err := client.CreateAuthSessionWithResponse(context.Background(), openapi.CreateAuthSessionJSONRequestBody{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("POST /v1/auth/sessions invalid: %v", err)
	}
	requireStatus(t, "POST /v1/auth/sessions invalid", response.StatusCode(), http.StatusUnauthorized, response.Body)
	if response.JSON401 == nil {
		t.Fatal("POST /v1/auth/sessions invalid returned 401 without JSON body")
	}
	if response.JSON401.Code != openapi.ErrorCodeInvalidAuth {
		t.Fatalf("invalid auth code = %s, want %s", response.JSON401.Code, openapi.ErrorCodeInvalidAuth)
	}
}

func getAuthSession(t *testing.T, client *openapi.ClientWithResponses) openapi.CurrentSessionResponse {
	t.Helper()

	response, err := client.GetAuthSessionWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/auth/session: %v", err)
	}
	requireStatus(t, "GET /v1/auth/session", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/auth/session returned 200 without JSON body")
	}
	return *response.JSON200
}

func requireUnauthenticatedAuthSession(t *testing.T, client *openapi.ClientWithResponses) {
	t.Helper()

	response, err := client.GetAuthSessionWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/auth/session unauthenticated: %v", err)
	}
	requireStatus(t, "GET /v1/auth/session unauthenticated", response.StatusCode(), http.StatusUnauthorized, response.Body)
	if response.JSON401 == nil {
		t.Fatal("GET /v1/auth/session unauthenticated returned 401 without JSON body")
	}
	if response.JSON401.Code != openapi.ErrorCodeUnauthenticated {
		t.Fatalf("unauthenticated code = %s, want %s", response.JSON401.Code, openapi.ErrorCodeUnauthenticated)
	}
}

func deleteAuthSession(t *testing.T, client *openapi.ClientWithResponses) {
	t.Helper()

	response, err := client.DeleteAuthSessionWithResponse(context.Background())
	if err != nil {
		t.Fatalf("DELETE /v1/auth/session: %v", err)
	}
	requireStatus(t, "DELETE /v1/auth/session", response.StatusCode(), http.StatusNoContent, response.Body)
}

func bearerTokenEditor(token string) openapi.RequestEditorFn {
	return func(_ context.Context, request *http.Request) error {
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func getNote(t *testing.T, client *openapi.ClientWithResponses, id string) openapi.Note {
	t.Helper()

	response, err := client.GetNoteWithResponse(context.Background(), id)
	if err != nil {
		t.Fatalf("GET /v1/notes/{note_id}: %v", err)
	}
	requireStatus(t, "GET /v1/notes/{note_id}", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/notes/{note_id} returned 200 without JSON body")
	}
	return *response.JSON200
}

func searchNotes(t *testing.T, client *openapi.ClientWithResponses, query string) openapi.SearchNotesResponse {
	t.Helper()

	response, err := client.SearchNotesWithResponse(context.Background(), &openapi.SearchNotesParams{Q: &query})
	if err != nil {
		t.Fatalf("GET /v1/search/notes: %v", err)
	}
	requireStatus(t, "GET /v1/search/notes", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/search/notes returned 200 without JSON body")
	}
	requireSearchResponseProvenance(t, *response.JSON200)
	return *response.JSON200
}

func searchNotesByCategory(t *testing.T, client *openapi.ClientWithResponses, query string, category string) openapi.SearchNotesResponse {
	t.Helper()

	categorySlug := openapi.CategorySlug(category)
	response, err := client.SearchNotesWithResponse(context.Background(), &openapi.SearchNotesParams{
		CategorySlug: &categorySlug,
		Q:            &query,
	})
	if err != nil {
		t.Fatalf("GET /v1/search/notes?q=%s&category_slug=%s: %v", query, category, err)
	}
	requireStatus(t, "GET /v1/search/notes?category_slug", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/search/notes?category_slug returned 200 without JSON body")
	}
	requireSearchResponseProvenance(t, *response.JSON200)
	return *response.JSON200
}
func requireSearchResponseProvenance(t *testing.T, response openapi.SearchNotesResponse) {
	t.Helper()
	if response.SearchVersion != openapi.SearchVersion("hybrid-serafim100m-fts5-v1") {
		t.Fatalf("search version = %q, want hybrid-serafim100m-fts5-v1", response.SearchVersion)
	}
	for index, result := range response.Results {
		switch result.RetrievalSource {
		case openapi.Lexical, openapi.Semantic, openapi.Hybrid:
		default:
			t.Fatalf("search result %d source = %q, want lexical, semantic, or hybrid", index, result.RetrievalSource)
		}
	}
}

func requireSearchResultByID(t *testing.T, response openapi.SearchNotesResponse, wantID string) openapi.SearchNoteResult {
	t.Helper()

	for _, result := range response.Results {
		if result.Note.Id == wantID {
			return result
		}
	}
	t.Fatalf("search note id %q missing from results", wantID)
	return openapi.SearchNoteResult{}
}

func requireLexicalMatch(t *testing.T, response openapi.SearchNotesResponse, wantID string) openapi.SearchNoteResult {
	t.Helper()

	result := requireSearchResultByID(t, response, wantID)
	if result.RetrievalSource != openapi.Lexical && result.RetrievalSource != openapi.Hybrid {
		t.Fatalf("note %q retrieval source = %q, want lexical or hybrid", wantID, result.RetrievalSource)
	}
	return result
}

func requireNeverLexicallyMatched(t *testing.T, response openapi.SearchNotesResponse, excludedID string) {
	t.Helper()

	for _, result := range response.Results {
		if result.Note.Id != excludedID {
			continue
		}
		if result.RetrievalSource != openapi.Semantic {
			t.Fatalf("note %q retrieval source = %q, want semantic-only", excludedID, result.RetrievalSource)
		}
	}
}

func requireOnlySemanticMatches(t *testing.T, response openapi.SearchNotesResponse, query string) {
	t.Helper()

	for _, result := range response.Results {
		if result.RetrievalSource != openapi.Semantic {
			t.Fatalf("query %q matched note %q with source %q, want semantic-only", query, result.Note.Id, result.RetrievalSource)
		}
	}
}

func requireListNotesCategoryFilterError(t *testing.T, client *openapi.ClientWithResponses, category string) {
	t.Helper()

	categorySlug := openapi.CategorySlug(category)
	response, err := client.ListNotesWithResponse(context.Background(), &openapi.ListNotesParams{CategorySlug: &categorySlug})
	if err != nil {
		t.Fatalf("GET /v1/notes?category_slug=%s: %v", category, err)
	}
	requireStatus(t, "GET /v1/notes?category_slug", response.StatusCode(), http.StatusBadRequest, response.Body)
	if response.JSON400 == nil {
		t.Fatal("GET /v1/notes?category_slug returned 400 without JSON body")
	}
	requireCategorySlugUnknownError(t, *response.JSON400, openapi.ErrorCodeInvalidNote)
}

func requireSearchNotesCategoryFilterError(t *testing.T, client *openapi.ClientWithResponses, category string) {
	t.Helper()

	categorySlug := openapi.CategorySlug(category)
	query := "balcao"
	response, err := client.SearchNotesWithResponse(context.Background(), &openapi.SearchNotesParams{
		CategorySlug: &categorySlug,
		Q:            &query,
	})
	if err != nil {
		t.Fatalf("GET /v1/search/notes?category_slug=%s: %v", category, err)
	}
	requireStatus(t, "GET /v1/search/notes?category_slug", response.StatusCode(), http.StatusBadRequest, response.Body)
	if response.JSON400 == nil {
		t.Fatal("GET /v1/search/notes?category_slug returned 400 without JSON body")
	}
	requireCategorySlugUnknownError(t, *response.JSON400, openapi.ErrorCodeInvalidSearch)
}

func requireCategorySlugUnknownError(t *testing.T, got openapi.ErrorResponse, wantCode openapi.ErrorCode) {
	t.Helper()

	if got.Code != wantCode {
		t.Fatalf("code = %s, want %s", got.Code, wantCode)
	}
	if got.Fields == nil {
		t.Fatal("fields = nil, want category_slug unknown")
	}
	wantFields := []openapi.ValidationProblem{{
		Field: openapi.ValidationFieldCategorySlug,
		Code:  openapi.ValidationProblemCodeUnknown,
	}}
	if diff := cmp.Diff(wantFields, *got.Fields); diff != "" {
		t.Fatalf("validation fields mismatch (-want +got):\n%s", diff)
	}
}

func requireStatus(t *testing.T, operation string, got int, want int, body []byte) {
	t.Helper()

	if got != want {
		t.Fatalf("%s status = %d, want %d; body: %s", operation, got, want, string(body))
	}
}

func requireAuthSession(t *testing.T, got openapi.AuthSessionResponse, username string, displayName string) {
	t.Helper()

	if got.Token == "" {
		t.Fatal("auth token is empty")
	}
	if got.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatalf("expires_at = %d, want future timestamp", got.ExpiresAt)
	}
	if got.User.Id == "" {
		t.Fatal("current user id is empty")
	}
	if got.User.Id == got.User.Author.Id {
		t.Fatal("current private user id matches public author id")
	}
	if got.User.Username != username {
		t.Fatalf("username = %q, want %q", got.User.Username, username)
	}
	if got.User.Author.Id == "" {
		t.Fatal("author id is empty")
	}
	if got.User.Author.DisplayName != displayName {
		t.Fatalf("author display_name = %q, want %q", got.User.Author.DisplayName, displayName)
	}
}

func requireCurrentSession(t *testing.T, got openapi.CurrentSessionResponse, want openapi.AuthSessionResponse) {
	t.Helper()

	if got.ExpiresAt != want.ExpiresAt {
		t.Fatalf("current expires_at = %d, want %d", got.ExpiresAt, want.ExpiresAt)
	}
	if diff := cmp.Diff(want.User, got.User); diff != "" {
		t.Fatalf("current user mismatch (-want +got):\n%s", diff)
	}
}

func requireCreatedNote(t *testing.T, got openapi.Note, want openapi.CreateNoteRequest) {
	t.Helper()

	if got.Id == "" {
		t.Fatal("note id is empty")
	}

	gotFields := noteFieldsFromResponse(got)
	wantFields := noteFieldsFromRequest(want)
	if diff := cmp.Diff(wantFields, gotFields); diff != "" {
		t.Fatalf("note fields mismatch (-want +got):\n%s", diff)
	}
	if got.CreatedAt <= 0 {
		t.Fatalf("created_at = %d, want positive timestamp", got.CreatedAt)
	}
	if got.UpdatedAt <= 0 {
		t.Fatalf("updated_at = %d, want positive timestamp", got.UpdatedAt)
	}
}

func requireListedNote(t *testing.T, notes openapi.ListNotesResponse, id string, want openapi.CreateNoteRequest) {
	t.Helper()

	for _, listedNote := range notes.Notes {
		if listedNote.Id == id {
			requireCreatedNote(t, listedNote, want)
			return
		}
	}

	t.Fatalf("listed note id %q missing", id)
}

func noteFieldsFromResponse(note openapi.Note) noteFields {
	return noteFields{
		Title:        note.Title,
		Body:         note.Body,
		CategorySlug: note.CategorySlug,
	}
}

func noteFieldsFromRequest(request openapi.CreateNoteRequest) noteFields {
	return noteFields{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: request.CategorySlug,
	}
}

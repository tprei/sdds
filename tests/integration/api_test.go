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
	PlaceSlug    *string
}

func TestAPIRuntimeBoundaries(t *testing.T) {
	client := newAPIClient(t)

	waitForReadiness(t, client)

	health, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	requireStatus(t, "GET /healthz", health.StatusCode(), http.StatusNoContent, health.Body)

	requireCatalogs(t, client)

	initialNotes := listNotes(t, client)
	if len(initialNotes.Notes) != 0 {
		t.Fatalf("initial note count = %d, want 0", len(initialNotes.Notes))
	}

	selectedPlace := "sao-paulo"
	request := openapi.CreateNoteJSONRequestBody{
		Title:        "Café bom",
		Body:         "Tem pao de queijo decente e balcao simpatico.",
		CategorySlug: "food",
		PlaceSlug:    &selectedPlace,
	}
	created := createNote(t, client, request)
	requireCreatedNote(t, created, request)

	requestWithoutPlace := openapi.CreateNoteJSONRequestBody{
		Title:        "Dica sem lugar",
		Body:         "Serve para qualquer lugar mundial.",
		CategorySlug: "food",
	}
	createdWithoutPlace := createNote(t, client, requestWithoutPlace)
	requireCreatedNote(t, createdWithoutPlace, requestWithoutPlace)

	updatedNotes := listNotes(t, client)
	if len(updatedNotes.Notes) != 2 {
		t.Fatalf("updated note count = %d, want 2", len(updatedNotes.Notes))
	}
	requireListedNote(t, updatedNotes, created.Id, request)
	requireListedNote(t, updatedNotes, createdWithoutPlace.Id, requestWithoutPlace)

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

	fetchedWithoutPlace := getNote(t, client, createdWithoutPlace.Id)
	requireCreatedNote(t, fetchedWithoutPlace, requestWithoutPlace)
	if fetchedWithoutPlace.Id != createdWithoutPlace.Id {
		t.Fatalf("fetched note without place id = %q, want %q", fetchedWithoutPlace.Id, createdWithoutPlace.Id)
	}
	if fetchedWithoutPlace.PlaceSlug != nil {
		t.Fatalf("fetched note without place place_slug = %q, want nil", *fetchedWithoutPlace.PlaceSlug)
	}

	searchResults := searchNotes(t, client, "balcao")
	if len(searchResults.Notes) != 1 {
		t.Fatalf("search note count = %d, want 1", len(searchResults.Notes))
	}
	requireCreatedNote(t, searchResults.Notes[0], request)
	if searchResults.Notes[0].Id != created.Id {
		t.Fatalf("search note id = %q, want %q", searchResults.Notes[0].Id, created.Id)
	}

	searchResultsWithoutPlace := searchNotes(t, client, "mundial")
	if len(searchResultsWithoutPlace.Notes) != 1 {
		t.Fatalf("search note without place count = %d, want 1", len(searchResultsWithoutPlace.Notes))
	}
	requireCreatedNote(t, searchResultsWithoutPlace.Notes[0], requestWithoutPlace)
	if searchResultsWithoutPlace.Notes[0].Id != createdWithoutPlace.Id {
		t.Fatalf("search note without place id = %q, want %q", searchResultsWithoutPlace.Notes[0].Id, createdWithoutPlace.Id)
	}

	emptySearchResults := searchNotes(t, client, "necessaire")
	if len(emptySearchResults.Notes) != 0 {
		t.Fatalf("empty search note count = %d, want 0", len(emptySearchResults.Notes))
	}
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

	places, err := client.ListPlacesWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/places: %v", err)
	}
	requireStatus(t, "GET /v1/places", places.StatusCode(), http.StatusOK, places.Body)
	if places.JSON200 == nil {
		t.Fatal("GET /v1/places returned 200 without JSON body")
	}
	wantPlaces := []openapi.CatalogPlace{
		{Active: true, DisplayOrder: 10, Label: "São Paulo", Slug: "sao-paulo"},
		{Active: true, DisplayOrder: 20, Label: "Rio de Janeiro", Slug: "rio-de-janeiro"},
		{Active: true, DisplayOrder: 30, Label: "Lisboa", Slug: "lisboa"},
	}
	if diff := cmp.Diff(wantPlaces, places.JSON200.Places); diff != "" {
		t.Fatalf("places mismatch (-want +got):\n%s", diff)
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

	response, err := client.ListNotesWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/notes: %v", err)
	}
	requireStatus(t, "GET /v1/notes", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/notes returned 200 without JSON body")
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

func searchNotes(t *testing.T, client *openapi.ClientWithResponses, query string) openapi.ListNotesResponse {
	t.Helper()

	response, err := client.SearchNotesWithResponse(context.Background(), &openapi.SearchNotesParams{Q: &query})
	if err != nil {
		t.Fatalf("GET /v1/search/notes: %v", err)
	}
	requireStatus(t, "GET /v1/search/notes", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/search/notes returned 200 without JSON body")
	}
	return *response.JSON200
}

func requireStatus(t *testing.T, operation string, got int, want int, body []byte) {
	t.Helper()

	if got != want {
		t.Fatalf("%s status = %d, want %d; body: %s", operation, got, want, string(body))
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
		PlaceSlug:    note.PlaceSlug,
	}
}

func noteFieldsFromRequest(request openapi.CreateNoteRequest) noteFields {
	return noteFields{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: request.CategorySlug,
		PlaceSlug:    request.PlaceSlug,
	}
}

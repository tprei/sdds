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
		CategorySlug: "travel",
	}
	createdWithoutPlace := createNote(t, client, requestWithoutPlace)
	requireCreatedNote(t, createdWithoutPlace, requestWithoutPlace)

	updatedNotes := listNotes(t, client)
	if len(updatedNotes.Notes) != 2 {
		t.Fatalf("updated note count = %d, want 2", len(updatedNotes.Notes))
	}
	requireListedNote(t, updatedNotes, created.Id, request)
	requireListedNote(t, updatedNotes, createdWithoutPlace.Id, requestWithoutPlace)

	foodNotes := listNotesByCategory(t, client, "food")
	if len(foodNotes.Notes) != 1 {
		t.Fatalf("food note count = %d, want 1", len(foodNotes.Notes))
	}
	requireListedNote(t, foodNotes, created.Id, request)

	travelNotes := listNotesByCategory(t, client, "travel")
	if len(travelNotes.Notes) != 1 {
		t.Fatalf("travel note count = %d, want 1", len(travelNotes.Notes))
	}
	requireListedNote(t, travelNotes, createdWithoutPlace.Id, requestWithoutPlace)

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

	filteredSearchResults := searchNotesByCategory(t, client, "mundial", "travel")
	if len(filteredSearchResults.Notes) != 1 {
		t.Fatalf("filtered search note count = %d, want 1", len(filteredSearchResults.Notes))
	}
	requireCreatedNote(t, filteredSearchResults.Notes[0], requestWithoutPlace)

	emptyFilteredSearchResults := searchNotesByCategory(t, client, "mundial", "food")
	if len(emptyFilteredSearchResults.Notes) != 0 {
		t.Fatalf("empty filtered search note count = %d, want 0", len(emptyFilteredSearchResults.Notes))
	}

	emptySearchResults := searchNotes(t, client, "necessaire")
	if len(emptySearchResults.Notes) != 0 {
		t.Fatalf("empty search note count = %d, want 0", len(emptySearchResults.Notes))
	}

	accentRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "Pão ftsaccent48",
		Body:         "Massa boa.",
		CategorySlug: "food",
	}
	accentNote := createNote(t, client, accentRequest)
	accentResults := searchNotes(t, client, "pao ftsaccent48")
	requireOnlySearchNoteIDs(t, accentResults, []string{accentNote.Id})

	strictBothRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "strictcafe48 strictpao48",
		Body:         "Encontro certo.",
		CategorySlug: "food",
	}
	strictBothNote := createNote(t, client, strictBothRequest)
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:        "strictcafe48",
		Body:         "Falta o segundo termo.",
		CategorySlug: "food",
	})
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:        "strictpao48",
		Body:         "Falta o primeiro termo.",
		CategorySlug: "food",
	})
	strictResults := searchNotes(t, client, "strictcafe48 strictpao48")
	requireOnlySearchNoteIDs(t, strictResults, []string{strictBothNote.Id})

	titleRankRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "rankbolo48 roteiro enorme com muitas palavras extras para alongar o titulo e reduzir relevancia sem peso",
		Body:         "Nota mais antiga.",
		CategorySlug: "food",
	}
	titleRankNote := createNote(t, client, titleRankRequest)
	bodyRankRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "Bolo curto",
		Body:         "rankbolo48.",
		CategorySlug: "food",
	}
	bodyRankNote := createNote(t, client, bodyRankRequest)
	rankedResults := searchNotes(t, client, "rankbolo48")
	requireOnlySearchNoteIDs(t, rankedResults, []string{titleRankNote.Id, bodyRankNote.Id})

	categoryFoodRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "catbusca48 comida",
		Body:         "Filtro de categoria.",
		CategorySlug: "food",
	}
	categoryFoodNote := createNote(t, client, categoryFoodRequest)
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:        "catbusca48 viagem",
		Body:         "Mesmo termo fora da categoria.",
		CategorySlug: "travel",
	})
	categoryResults := searchNotesByCategory(t, client, "catbusca48", "food")
	requireOnlySearchNoteIDs(t, categoryResults, []string{categoryFoodNote.Id})

	globalPlace := "sao-paulo"
	globalWithPlaceRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "globalbusca48 com lugar",
		Body:         "Aparece na busca global.",
		CategorySlug: "travel",
		PlaceSlug:    &globalPlace,
	}
	globalWithPlaceNote := createNote(t, client, globalWithPlaceRequest)
	globalWithoutPlaceRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "globalbusca48 sem lugar",
		Body:         "Tambem aparece na busca global.",
		CategorySlug: "travel",
	}
	globalWithoutPlaceNote := createNote(t, client, globalWithoutPlaceRequest)
	globalResults := searchNotes(t, client, "globalbusca48")
	requireSearchNoteIDs(t, globalResults, []string{globalWithPlaceNote.Id, globalWithoutPlaceNote.Id})

	punctuationRequest := openapi.CreateNoteJSONRequestBody{
		Title:        "pontoseguro48",
		Body:         "Pontuacao nao muda a busca.",
		CategorySlug: "food",
	}
	punctuationNote := createNote(t, client, punctuationRequest)
	punctuationResults := searchNotes(t, client, "pontoseguro48 ***")
	requireOnlySearchNoteIDs(t, punctuationResults, []string{punctuationNote.Id})

	punctuationOnlyResults := searchNotes(t, client, "!!! *** ()")
	if len(punctuationOnlyResults.Notes) != 0 {
		t.Fatalf("punctuation-only search note count = %d, want 0", len(punctuationOnlyResults.Notes))
	}

	requireListNotesCategoryFilterError(t, client, "comida")
	requireSearchNotesCategoryFilterError(t, client, "comida")
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

func searchNotesByCategory(t *testing.T, client *openapi.ClientWithResponses, query string, category string) openapi.ListNotesResponse {
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
	return *response.JSON200
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

func requireOnlySearchNoteIDs(t *testing.T, notes openapi.ListNotesResponse, wantIDs []string) {
	t.Helper()

	gotIDs := make([]string, 0, len(notes.Notes))
	for _, found := range notes.Notes {
		gotIDs = append(gotIDs, found.Id)
	}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func requireSearchNoteIDs(t *testing.T, notes openapi.ListNotesResponse, wantIDs []string) {
	t.Helper()

	if len(notes.Notes) != len(wantIDs) {
		t.Fatalf("search note count = %d, want %d", len(notes.Notes), len(wantIDs))
	}

	gotIDs := make(map[string]bool, len(notes.Notes))
	for _, found := range notes.Notes {
		gotIDs[found.Id] = true
	}
	for _, wantID := range wantIDs {
		if !gotIDs[wantID] {
			t.Fatalf("search note id %q missing", wantID)
		}
	}
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

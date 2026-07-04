//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

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
	CitySlug     string
}

func TestAPIRuntimeBoundaries(t *testing.T) {
	client := newAPIClient(t)

	waitForReadiness(t, client)

	health, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	requireStatus(t, "GET /healthz", health.StatusCode(), http.StatusNoContent, health.Body)

	initialNotes := listNotes(t, client)
	if len(initialNotes.Notes) != 0 {
		t.Fatalf("initial note count = %d, want 0", len(initialNotes.Notes))
	}

	request := openapi.CreateNoteJSONRequestBody{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	}
	created := createNote(t, client, request)
	requireCreatedNote(t, created, request)

	updatedNotes := listNotes(t, client)
	if len(updatedNotes.Notes) != 1 {
		t.Fatalf("updated note count = %d, want 1", len(updatedNotes.Notes))
	}
	requireCreatedNote(t, updatedNotes.Notes[0], request)
	if updatedNotes.Notes[0].Id != created.Id {
		t.Fatalf("listed note id = %q, want %q", updatedNotes.Notes[0].Id, created.Id)
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
	if gotFields != wantFields {
		t.Fatalf("note fields = %#v, want %#v", gotFields, wantFields)
	}
	if got.CreatedAt <= 0 {
		t.Fatalf("created_at = %d, want positive timestamp", got.CreatedAt)
	}
	if got.UpdatedAt <= 0 {
		t.Fatalf("updated_at = %d, want positive timestamp", got.UpdatedAt)
	}
}

func noteFieldsFromResponse(note openapi.Note) noteFields {
	return noteFields{
		Title:        note.Title,
		Body:         note.Body,
		CategorySlug: note.CategorySlug,
		CitySlug:     note.CitySlug,
	}
}

func noteFieldsFromRequest(request openapi.CreateNoteRequest) noteFields {
	return noteFields{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: request.CategorySlug,
		CitySlug:     request.CitySlug,
	}
}

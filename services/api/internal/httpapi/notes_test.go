package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

func TestListNotesReturnsRecentNotes(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := NewRouter(fakeNoteStore{
		listNotes: func(_ context.Context, limit int) ([]note.Note, error) {
			if limit != recentNotesLimit {
				t.Fatalf("limit = %d, want %d", limit, recentNotesLimit)
			}
			return []note.Note{{
				ID:           "018ff5b8-0000-7000-8000-000000000000",
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: "comida",
				CitySlug:     "sao-paulo",
				CreatedAt:    now,
				UpdatedAt:    now,
			}}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body listNotesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Notes) != 1 {
		t.Fatalf("note count = %d, want 1", len(body.Notes))
	}
	if body.Notes[0].Category != "comida" {
		t.Fatalf("category = %s, want comida", body.Notes[0].Category)
	}
}

func TestCreateNoteReturnsCreatedNote(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := NewRouter(fakeNoteStore{
		createNote: func(_ context.Context, input note.CreateInput) (note.Note, error) {
			if input.Title != "Café bom" {
				t.Fatalf("title = %q, want Café bom", input.Title)
			}
			if input.CategorySlug != "comida" {
				t.Fatalf("category = %q, want comida", input.CategorySlug)
			}
			return note.Note{
				ID:           "018ff5b8-0000-7000-8000-000000000000",
				Title:        input.Title,
				Body:         input.Body,
				CategorySlug: input.CategorySlug,
				CitySlug:     input.CitySlug,
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		},
	})

	requestBody := []byte(`{"title":" Café bom ","body":"Tem pão de queijo decente.","category":"comida","city":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body noteResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID == "" {
		t.Fatal("id is empty")
	}
	if body.City != "sao-paulo" {
		t.Fatalf("city = %s, want sao-paulo", body.City)
	}
}

func TestCreateNoteRejectsValidationProblems(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"","body":"Funciona.","category":"qualquer","city":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "invalid_note" {
		t.Fatalf("error = %s, want invalid_note", body.Error)
	}
	if len(body.Fields) != 2 {
		t.Fatalf("field count = %d, want 2", len(body.Fields))
	}
}

func TestCreateNoteRejectsInvalidJSON(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader([]byte(`{"title":`)))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCreateNoteRejectsTrailingJSON(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category":"comida","city":"sao-paulo"} {}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCreateNoteRejectsOversizedRequestBody(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"` + strings.Repeat("a", maxCreateNoteRequestBytes) + `","category":"comida","city":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "request_too_large" {
		t.Fatalf("error = %s, want request_too_large", body.Error)
	}
}

func TestNoteRoutesRejectUnsupportedMethods(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

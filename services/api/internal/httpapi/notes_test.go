package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const exampleNoteID = "018ff5b8-0000-7000-8000-000000000000"

func TestListNotesReturnsRecentNotes(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := newTestRouter(fakeNoteStore{
		listNotes: func(_ context.Context, input note.ListInput) ([]note.Note, error) {
			if input.Limit != recentNotesLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, recentNotesLimit)
			}
			if input.CategorySlug != "" {
				t.Fatalf("category slug = %q, want empty", input.CategorySlug)
			}
			return []note.Note{{
				ID:           exampleNoteID,
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: "food",
				PlaceSlug:    "sao-paulo",
				CreatedAt:    now,
				UpdatedAt:    now,
			}}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListNotesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.ListNotesResponse{Notes: []openapi.Note{{
		Id:           exampleNoteID,
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: string(note.CategorySlugFood),
		PlaceSlug:    stringPointer(string(note.PlaceSlugSaoPaulo)),
		Images:       []openapi.NoteImage{},
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}}}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	notesValue, ok := wireBody["notes"].([]any)
	if !ok {
		t.Fatalf("notes = %T, want []any", wireBody["notes"])
	}
	if len(notesValue) != 1 {
		t.Fatalf("wire note count = %d, want 1", len(notesValue))
	}

	noteValue, ok := notesValue[0].(map[string]any)
	if !ok {
		t.Fatalf("note = %T, want map[string]any", notesValue[0])
	}
	requireJSONKeys(t, noteValue, "id", "title", "body", "category_slug", "place_slug", "images", "created_at", "updated_at")
	requireJSONNumber(t, noteValue, "created_at", now.UnixMilli())
	requireJSONNumber(t, noteValue, "updated_at", now.UnixMilli())
}

func TestListNotesFiltersByCategory(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(_ context.Context, input note.ListInput) ([]note.Note, error) {
			if input.CategorySlug != note.CategorySlugFood {
				t.Fatalf("category slug = %q, want %q", input.CategorySlug, note.CategorySlugFood)
			}
			if input.Limit != recentNotesLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, recentNotesLimit)
			}
			return []note.Note{}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes?category_slug=+food+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestListNotesTreatsBlankCategoryFilterAsUnfiltered(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		listNotes: func(_ context.Context, input note.ListInput) ([]note.Note, error) {
			if input.CategorySlug != "" {
				t.Fatalf("category slug = %q, want empty", input.CategorySlug)
			}
			return []note.Note{}, nil
		},
	}, fakeCatalog{
		findActiveCategory: func(context.Context, note.CategorySlug) (note.Category, error) {
			t.Fatal("FindActiveCategory should not be called")
			return note.Category{}, nil
		},
	}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes?category_slug=+%09+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestListNotesRejectsUnknownCategoryFilter(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(context.Context, note.ListInput) ([]note.Note, error) {
			t.Fatal("ListRecentNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes?category_slug=comida", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldCategorySlug, Code: openapi.ValidationProblemCodeUnknown},
	})
}

func TestListNotesRejectsDuplicateCategoryFilter(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(context.Context, note.ListInput) ([]note.Note, error) {
			t.Fatal("ListRecentNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes?category_slug=food&category_slug=travel", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
	}
	if body.Fields != nil {
		t.Fatalf("fields = %#v, want nil", *body.Fields)
	}
}

func TestSearchNotesReturnsMatchingNotes(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(_ context.Context, input note.SearchInput) ([]note.Note, error) {
			if input.Query != "café" {
				t.Fatalf("query = %q, want café", input.Query)
			}
			if input.Limit != searchNotesLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, searchNotesLimit)
			}
			if input.CategorySlug != "" {
				t.Fatalf("category slug = %q, want empty", input.CategorySlug)
			}
			return []note.Note{{
				ID:           exampleNoteID,
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: "food",
				PlaceSlug:    "sao-paulo",
				CreatedAt:    now,
				UpdatedAt:    now,
			}}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=+caf%C3%A9+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListNotesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.ListNotesResponse{Notes: []openapi.Note{{
		Id:           exampleNoteID,
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: string(note.CategorySlugFood),
		PlaceSlug:    stringPointer(string(note.PlaceSlugSaoPaulo)),
		Images:       []openapi.NoteImage{},
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}}}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}
}

func TestSearchNotesReturnsEmptyNotesForPunctuationOnlyQuery(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(_ context.Context, input note.SearchInput) ([]note.Note, error) {
			if input.Query != "!!! *** ()" {
				t.Fatalf("query = %q, want punctuation-only query", input.Query)
			}
			if input.Limit != searchNotesLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, searchNotesLimit)
			}
			if input.CategorySlug != "" {
				t.Fatalf("category slug = %q, want empty", input.CategorySlug)
			}
			return []note.Note{}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=!!!+***+()", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListNotesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Notes) != 0 {
		t.Fatalf("note count = %d, want 0", len(body.Notes))
	}
}

func TestSearchNotesFiltersByCategory(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(_ context.Context, input note.SearchInput) ([]note.Note, error) {
			if input.Query != "café" {
				t.Fatalf("query = %q, want café", input.Query)
			}
			if input.CategorySlug != note.CategorySlugFood {
				t.Fatalf("category slug = %q, want %q", input.CategorySlug, note.CategorySlugFood)
			}
			if input.Limit != searchNotesLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, searchNotesLimit)
			}
			return []note.Note{}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=caf%C3%A9&category_slug=+food+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestSearchNotesTreatsBlankCategoryFilterAsUnfiltered(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		searchNotes: func(_ context.Context, input note.SearchInput) ([]note.Note, error) {
			if input.CategorySlug != "" {
				t.Fatalf("category slug = %q, want empty", input.CategorySlug)
			}
			return []note.Note{}, nil
		},
	}, fakeCatalog{
		findActiveCategory: func(context.Context, note.CategorySlug) (note.Category, error) {
			t.Fatal("FindActiveCategory should not be called")
			return note.Category{}, nil
		},
	}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=cafe&category_slug=+%09+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestSearchNotesRejectsUnknownCategoryFilter(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=cafe&category_slug=comida", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldCategorySlug, Code: openapi.ValidationProblemCodeUnknown},
	})
}

func TestSearchNotesRejectsEmptyQuery(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=+%09+", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldQ, Code: openapi.ValidationProblemCodeRequired},
	})
}

func TestSearchNotesRejectsMissingQuery(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldQ, Code: openapi.ValidationProblemCodeRequired},
	})
}

func TestSearchNotesRejectsLongQuery(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q="+strings.Repeat("a", note.SearchQueryMaxLength+1), nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldQ, Code: openapi.ValidationProblemCodeTooLong},
	})
}

func TestSearchNotesRejectsDuplicateQuery(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=cafe&q=pao", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	if body.Fields != nil {
		t.Fatalf("fields = %#v, want nil", *body.Fields)
	}
}

func TestSearchNotesRejectsDuplicateCategoryFilter(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			t.Fatal("SearchNotes should not be called")
			return nil, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=cafe&category_slug=food&category_slug=travel", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidSearch {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidSearch)
	}
	if body.Fields != nil {
		t.Fatalf("fields = %#v, want nil", *body.Fields)
	}
}

func TestSearchNotesReturnsInternalError(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			return nil, errors.New("database unavailable")
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search/notes?q=caf%C3%A9", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInternal {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInternal)
	}
}

func TestGetNoteReturnsNote(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := newTestRouter(fakeNoteStore{
		findNote: func(_ context.Context, id string) (note.Note, error) {
			if id != exampleNoteID {
				t.Fatalf("id = %q, want note id", id)
			}
			return note.Note{
				ID:           id,
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: "food",
				PlaceSlug:    "sao-paulo",
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/"+exampleNoteID, nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.Note{
		Id:           exampleNoteID,
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: string(note.CategorySlugFood),
		Images:       []openapi.NoteImage{},
		PlaceSlug:    stringPointer(string(note.PlaceSlugSaoPaulo)),
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}
}

func TestGetNoteReturnsNotFound(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		findNote: func(context.Context, string) (note.Note, error) {
			return note.Note{}, note.ErrNoteNotFound
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/missing-note", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeNotFound {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeNotFound)
	}
}

func TestGetNoteReturnsInternalError(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		findNote: func(context.Context, string) (note.Note, error) {
			return note.Note{}, errors.New("database unavailable")
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/"+exampleNoteID, nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInternal {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInternal)
	}
}

func TestCreateNoteReturnsCreatedNote(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := newTestRouter(fakeNoteStore{
		createNote: func(_ context.Context, input note.CreateInput) (note.Note, error) {
			if input.UserID != user.UserID("user-id-thiago") {
				t.Fatalf("user id = %q, want user-id-thiago", input.UserID)
			}
			if input.Title != "Café bom" {
				t.Fatalf("title = %q, want Café bom", input.Title)
			}
			if input.CategorySlug != "food" {
				t.Fatalf("category = %q, want food", input.CategorySlug)
			}
			return note.Note{
				ID:           exampleNoteID,
				Title:        input.Title,
				Body:         input.Body,
				CategorySlug: input.CategorySlug,
				PlaceSlug:    input.PlaceSlug,
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		},
	})

	requestBody := []byte(`{"title":" Café bom ","body":"Tem pão de queijo decente.","category_slug":"food","place_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.Note{
		Id:           exampleNoteID,
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: string(note.CategorySlugFood),
		PlaceSlug:    stringPointer(string(note.PlaceSlugSaoPaulo)),
		Images:       []openapi.NoteImage{},
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireJSONNumber(t, wireBody, "created_at", now.UnixMilli())
	requireJSONNumber(t, wireBody, "updated_at", now.UnixMilli())
}

func TestCreateNoteRejectsMissingSessionBeforeValidation(t *testing.T) {
	createCalled := false
	router := NewRouter(fakeNoteStore{
		createNote: func(_ context.Context, _ note.CreateInput) (note.Note, error) {
			createCalled = true
			return note.Note{}, nil
		},
	}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{})
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
	if createCalled {
		t.Fatal("note store was called")
	}
}

func TestCreateNoteAcceptsOmittedPlace(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		createNote: func(_ context.Context, input note.CreateInput) (note.Note, error) {
			if input.PlaceSlug != "" {
				t.Fatalf("place slug = %q, want empty", input.PlaceSlug)
			}
			return note.Note{
				ID:           exampleNoteID,
				Title:        input.Title,
				Body:         input.Body,
				CategorySlug: input.CategorySlug,
				CreatedAt:    time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(`{"title":"Café bom","body":"Funciona.","category_slug":"food"}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PlaceSlug != nil {
		t.Fatalf("place_slug = %q, want nil", *body.PlaceSlug)
	}
}

func TestCreateNoteAcceptsNullPlace(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		createNote: func(_ context.Context, input note.CreateInput) (note.Note, error) {
			if input.PlaceSlug != "" {
				t.Fatalf("place slug = %q, want empty", input.PlaceSlug)
			}
			return note.Note{
				ID:           exampleNoteID,
				Title:        input.Title,
				Body:         input.Body,
				CategorySlug: input.CategorySlug,
				CreatedAt:    time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(`{"title":"Café bom","body":"Funciona.","category_slug":"food","place_slug":null}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PlaceSlug != nil {
		t.Fatalf("place_slug = %q, want nil", *body.PlaceSlug)
	}
}

func TestCreateNoteRejectsValidationProblems(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"   ","body":"   ","category_slug":"food","place_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldTitle, Code: openapi.ValidationProblemCodeRequired},
		{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeRequired},
	})

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireJSONKeys(t, wireBody, "code", "fields")
	if _, ok := wireBody["error"]; ok {
		t.Fatal("unexpected error key in response body")
	}

	fieldsValue, ok := wireBody["fields"].([]any)
	if !ok {
		t.Fatalf("fields = %T, want []any", wireBody["fields"])
	}
	if len(fieldsValue) != 2 {
		t.Fatalf("wire field count = %d, want 2", len(fieldsValue))
	}

	firstField, ok := fieldsValue[0].(map[string]any)
	if !ok {
		t.Fatalf("first field = %T, want map[string]any", fieldsValue[0])
	}
	requireJSONKeys(t, firstField, "field", "code")
}

func TestCreateNoteRejectsOpenAPIRequestSchemaProblems(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "title too long",
			body: `{"title":"` + strings.Repeat("a", note.TitleMaxLength+1) + `","body":"Funciona.","category_slug":"food","place_slug":"sao-paulo"}`,
		},
		{
			name: "body too long",
			body: `{"title":"Café bom","body":"` + strings.Repeat("a", note.BodyMaxLength+1) + `","category_slug":"food","place_slug":"sao-paulo"}`,
		},
	}

	router := newTestRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidJSON {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
			}
			if body.Fields != nil {
				t.Fatalf("fields = %#v, want nil", *body.Fields)
			}
		})
	}
}

func TestCreateNoteRejectsUnknownSlugsThroughCatalogValidation(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"qualquer","place_slug":"qualquer"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldCategorySlug, Code: openapi.ValidationProblemCodeUnknown},
		{Field: openapi.ValidationFieldPlaceSlug, Code: openapi.ValidationProblemCodeUnknown},
	})
}

func TestCreateNoteRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader([]byte(`{"title":`)))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsMissingOrUnsupportedContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "missing"},
		{name: "unsupported", contentType: "text/plain"},
	}

	router := newTestRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := `{"title":"Café bom","body":"Funciona.","category_slug":"food","place_slug":"sao-paulo"}`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(requestBody))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidJSON {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
			}
		})
	}
}

func TestCreateNoteRejectsOldCitySlugJSON(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"food","city_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsUnknownJSONFields(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"food","place_slug":"sao-paulo","unexpected":true}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsTrailingJSON(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"food","place_slug":"sao-paulo"} {}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsOversizedRequestBody(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"` + strings.Repeat("a", int(maxCreateNoteRequestBytes)) + `","category_slug":"food","place_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeRequestTooLarge)
	}
}

func TestListNotesRejectsRequestBody(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(context.Context, note.ListInput) ([]note.Note, error) {
			t.Fatal("ListRecentNotes should not be called")
			return nil, nil
		},
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", strings.NewReader(`{"unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestNoteRoutesRejectUnsupportedMethods(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func requireOpenAPIResponse(t *testing.T, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()

	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("load OpenAPI spec: %v", err)
	}
	spec.Servers = nil

	router, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatalf("build OpenAPI router: %v", err)
	}

	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route: %v", err)
	}

	options := &openapi3filter.Options{
		AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		IncludeResponseStatus: true,
	}
	err = openapi3filter.ValidateResponse(request.Context(), (&openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: pathParams,
			Route:      route,
			Options:    options,
		},
		Status:  response.Code,
		Header:  response.Header(),
		Options: options,
	}).SetBodyBytes(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("response does not match OpenAPI contract: %v", err)
	}
}

func decodeResponseObject(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode wire response: %v", err)
	}
	return value
}

func requireJSONNumber(t *testing.T, value map[string]any, key string, want int64) {
	t.Helper()

	got, ok := value[key].(float64)
	if !ok {
		t.Fatalf("%s = %T, want JSON number", key, value[key])
	}
	if got != float64(want) {
		t.Fatalf("%s = %v, want %d", key, got, want)
	}
}

func requireValidationProblems(t *testing.T, got *[]openapi.ValidationProblem, want []openapi.ValidationProblem) {
	t.Helper()

	if got == nil {
		t.Fatal("fields is nil")
	}
	sortProblems := cmpopts.SortSlices(func(left openapi.ValidationProblem, right openapi.ValidationProblem) bool {
		if left.Field != right.Field {
			return left.Field < right.Field
		}
		return left.Code < right.Code
	})
	if diff := cmp.Diff(want, *got, sortProblems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}

func requireJSONKeys(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := value[key]; !ok {
			t.Fatalf("missing JSON key %q in %#v", key, value)
		}
	}
}

func stringPointer(value string) *string {
	return &value
}

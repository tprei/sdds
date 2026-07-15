package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func TestListCategoriesReturnsCatalogRows(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/categories", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListCategoriesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := newListCategoriesResponse(note.Categories)
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}
}

func TestListPlacesReturnsCatalogRows(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/places", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListPlacesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := newListPlacesResponse(note.Places)
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}
}

func TestListCategoriesReturnsInternalError(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listCategories: func(context.Context) ([]note.Category, error) {
			return nil, errors.New("catalog unavailable")
		},
	}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/categories", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestListPlacesReturnsInternalError(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listPlaces: func(context.Context) ([]note.Place, error) {
			return nil, errors.New("catalog unavailable")
		},
	}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/places", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

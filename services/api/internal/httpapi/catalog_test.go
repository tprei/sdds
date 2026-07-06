package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func TestListCategoriesReturnsCatalog(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listCategories: func(context.Context) ([]note.Category, error) {
			return []note.Category{
				{Slug: note.CategorySlugComida, Label: "Comida", Active: true, DisplayOrder: 20},
				{Slug: note.CategorySlugBeleza, Label: "Beleza", Active: false, DisplayOrder: 10},
			}, nil
		},
	})
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
	want := openapi.ListCategoriesResponse{
		Categories: []openapi.CatalogCategory{
			{Slug: "comida", Label: "Comida", Active: true, DisplayOrder: 20},
			{Slug: "beleza", Label: "Beleza", Active: false, DisplayOrder: 10},
		},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestListPlacesReturnsCatalog(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listPlaces: func(context.Context) ([]note.Place, error) {
			return []note.Place{
				{Slug: note.PlaceSlugSaoPaulo, Label: "São Paulo", Active: true, DisplayOrder: 10},
				{Slug: note.PlaceSlugLisboa, Label: "Lisboa", Active: false, DisplayOrder: 30},
			}, nil
		},
	})
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
	want := openapi.ListPlacesResponse{
		Places: []openapi.CatalogPlace{
			{Slug: "sao-paulo", Label: "São Paulo", Active: true, DisplayOrder: 10},
			{Slug: "lisboa", Label: "Lisboa", Active: false, DisplayOrder: 30},
		},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestListCategoriesHandlesStoreErrors(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listCategories: func(context.Context) ([]note.Category, error) {
			return nil, fmt.Errorf("catalog failed")
		},
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/categories", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestListPlacesHandlesStoreErrors(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{
		listPlaces: func(context.Context) ([]note.Place, error) {
			return nil, fmt.Errorf("catalog failed")
		},
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/places", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

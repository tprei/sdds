package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func TestListCategoriesReturnsCatalog(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
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
			{Slug: "beleza", Label: "Beleza", Active: true, DisplayOrder: 10},
			{Slug: "comida", Label: "Comida", Active: true, DisplayOrder: 20},
			{Slug: "viagem", Label: "Viagem", Active: true, DisplayOrder: 30},
			{Slug: "achadinhos", Label: "Achadinhos", Active: true, DisplayOrder: 40},
		},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestListPlacesReturnsCatalog(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
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
			{Slug: "rio-de-janeiro", Label: "Rio de Janeiro", Active: true, DisplayOrder: 20},
			{Slug: "lisboa", Label: "Lisboa", Active: true, DisplayOrder: 30},
		},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

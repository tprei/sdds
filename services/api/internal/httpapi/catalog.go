package httpapi

import (
	"net/http"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func (handler server) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := handler.catalog.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListCategoriesResponse(categories))
}

func (handler server) ListPlaces(w http.ResponseWriter, r *http.Request) {
	places, err := handler.catalog.ListPlaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListPlacesResponse(places))
}

func newListCategoriesResponse(categories []note.Category) openapi.ListCategoriesResponse {
	response := openapi.ListCategoriesResponse{Categories: make([]openapi.CatalogCategory, 0, len(categories))}
	for _, category := range categories {
		response.Categories = append(response.Categories, openapi.CatalogCategory{
			Slug:         openapi.CategorySlug(category.Slug),
			Label:        category.Label,
			Active:       category.Active,
			DisplayOrder: int32(category.DisplayOrder),
		})
	}
	return response
}

func newListPlacesResponse(places []note.Place) openapi.ListPlacesResponse {
	response := openapi.ListPlacesResponse{Places: make([]openapi.CatalogPlace, 0, len(places))}
	for _, place := range places {
		response.Places = append(response.Places, openapi.CatalogPlace{
			Slug:         openapi.PlaceSlug(place.Slug),
			Label:        place.Label,
			Active:       place.Active,
			DisplayOrder: int32(place.DisplayOrder),
		})
	}
	return response
}

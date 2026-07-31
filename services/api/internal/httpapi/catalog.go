package httpapi

import (
	"net/http"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func (handler server) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := handler.notes.catalog.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListCategoriesResponse(categories))
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

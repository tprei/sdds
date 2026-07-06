package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

type server struct {
	notes   note.Store
	catalog note.Catalog
}

var _ openapi.ServerInterface = server{}

func NewRouter(notes note.Store, catalog note.Catalog) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	router.Use(openAPIRequestValidator())

	return openapi.HandlerWithOptions(server{notes: notes, catalog: catalog}, openapi.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: writeGeneratedOpenAPIError,
	})
}

func writeGeneratedOpenAPIError(w http.ResponseWriter, _ *http.Request, err error) {
	var invalidParamError *openapi.InvalidParamFormatError
	if errors.As(err, &invalidParamError) && invalidParamError.ParamName == "q" {
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidSearch})
		return
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (server) GetHealth(w http.ResponseWriter, r *http.Request) {
	noContent(w, r)
}

func (server) GetReadiness(w http.ResponseWriter, r *http.Request) {
	noContent(w, r)
}

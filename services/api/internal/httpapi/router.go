package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

type server struct {
	notes note.Store
}

var _ openapi.ServerInterface = server{}

func NewRouter(notes note.Store) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	router.Use(openAPIRequestValidator())

	return openapi.HandlerFromMux(server{notes: notes}, router)
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

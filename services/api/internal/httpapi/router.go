package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/note"
)

func NewRouter(notes note.Store) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	router.Get("/healthz", noContent)
	router.Get("/readyz", noContent)

	noteHandler := noteHandler{notes: notes}
	router.Route("/v1", func(router chi.Router) {
		router.Get("/notes", noteHandler.listNotes)
		router.Post("/notes", noteHandler.createNote)
	})

	return router
}

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

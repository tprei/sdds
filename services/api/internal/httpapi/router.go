package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type server struct {
	notes           note.Store
	catalog         note.Catalog
	users           user.Store
	passwordHasher  passwordHasher
	credentialProbe string
	newSessionToken func() (string, error)
	clock           func() time.Time
}

var _ openapi.ServerInterface = server{}

type passwordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, encoded string) (bool, error)
}

func NewRouter(notes note.Store, catalog note.Catalog, users user.Store) http.Handler {
	hasher := user.NewPasswordHasher()
	return newRouter(notes, catalog, users, hasher, mustCredentialProbeHash(hasher), user.NewSessionToken, time.Now)
}

func newRouter(
	notes note.Store,
	catalog note.Catalog,
	users user.Store,
	passwordHasher passwordHasher,
	credentialProbe string,
	newSessionToken func() (string, error),
	clock func() time.Time,
) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	router.Use(openAPIRequestValidator())

	handler := server{
		notes:           notes,
		catalog:         catalog,
		users:           users,
		passwordHasher:  passwordHasher,
		credentialProbe: credentialProbe,
		newSessionToken: newSessionToken,
		clock:           clock,
	}
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: writeGeneratedOpenAPIError,
	}

	router.Get("/healthz", wrapper.GetHealth)
	router.Get("/readyz", wrapper.GetReadiness)
	router.Get("/v1/categories", wrapper.ListCategories)
	router.Get("/v1/places", wrapper.ListPlaces)
	router.Get("/v1/notes", wrapper.ListNotes)
	router.Post("/v1/notes", wrapper.CreateNote)
	router.Get("/v1/notes/{note_id}", wrapper.GetNote)
	router.Get("/v1/search/notes", wrapper.SearchNotes)
	router.Post("/v1/auth/users", wrapper.CreateAuthUser)
	router.Post("/v1/auth/sessions", wrapper.CreateAuthSession)
	router.With(requireAuth(users, clock)).Get("/v1/auth/session", wrapper.GetAuthSession)
	router.With(requireAuth(users, clock)).Delete("/v1/auth/session", wrapper.DeleteAuthSession)

	return router
}

func mustCredentialProbeHash(hasher passwordHasher) string {
	hash, err := hasher.Hash("invalid-credential-probe")
	if err != nil {
		panic(err)
	}
	return hash
}

func writeGeneratedOpenAPIError(w http.ResponseWriter, r *http.Request, err error) {
	var invalidParamError *openapi.InvalidParamFormatError
	if errors.As(err, &invalidParamError) {
		if code, ok := generatedInvalidParamErrorCode(r.URL.Path, invalidParamError.ParamName); ok {
			writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: code})
			return
		}
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func generatedInvalidParamErrorCode(path string, paramName string) (openapi.ErrorCode, bool) {
	switch {
	case path == "/v1/search/notes" && (paramName == "q" || paramName == "category_slug"):
		return openapi.ErrorCodeInvalidSearch, true
	case path == "/v1/notes" && paramName == "category_slug":
		return openapi.ErrorCodeInvalidNote, true
	default:
		return "", false
	}
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

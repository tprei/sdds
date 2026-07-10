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
	notes                 note.Store
	catalog               note.Catalog
	users                 user.Store
	passwordHasher        passwordHasher
	invalidCredentialHash string
	authRateLimiters      authRateLimiters
	newSessionToken       func() (string, error)
	clock                 func() time.Time
}

var _ openapi.ServerInterface = server{}

type passwordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, encoded string) (bool, error)
}

type AuthLimits struct {
	SignupRequestsPerMinute int
	LoginRequestsPerMinute  int
	PasswordHashConcurrency int
}

func DefaultAuthLimits() AuthLimits {
	return AuthLimits{
		SignupRequestsPerMinute: 5,
		LoginRequestsPerMinute:  10,
		PasswordHashConcurrency: 2,
	}
}

func NewRouter(notes note.Store, catalog note.Catalog, users user.Store, authLimits AuthLimits) http.Handler {
	hasher := newBoundedPasswordHasher(user.NewPasswordHasher(), authLimits.PasswordHashConcurrency)
	return newRouter(notes, catalog, users, hasher, mustInvalidCredentialHash(hasher), user.NewSessionToken, time.Now, authLimits)
}

func newRouter(
	notes note.Store,
	catalog note.Catalog,
	users user.Store,
	passwordHasher passwordHasher,
	invalidCredentialHash string,
	newSessionToken func() (string, error),
	clock func() time.Time,
	authLimits AuthLimits,
) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	router.Use(openAPIRequestValidator())

	authRateLimiters := newAuthRateLimiters(authLimits, clock)
	handler := server{
		notes:                 notes,
		catalog:               catalog,
		users:                 users,
		passwordHasher:        passwordHasher,
		invalidCredentialHash: invalidCredentialHash,
		authRateLimiters:      authRateLimiters,
		newSessionToken:       newSessionToken,
		clock:                 clock,
	}
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: writeGeneratedOpenAPIError,
	}

	router.Get("/healthz", wrapper.GetHealth)
	router.Get("/readyz", wrapper.GetReadiness)
	router.Route("/v1", func(router chi.Router) {
		router.Get("/categories", wrapper.ListCategories)
		router.Get("/places", wrapper.ListPlaces)
		router.Get("/notes", wrapper.ListNotes)
		router.Post("/notes", wrapper.CreateNote)
		router.Get("/notes/{note_id}", wrapper.GetNote)
		router.Get("/search/notes", wrapper.SearchNotes)
		router.With(authRateLimiters.signup).Post("/auth/users", wrapper.CreateAuthUser)
		router.With(authRateLimiters.login).Post("/auth/sessions", wrapper.CreateAuthSession)
		router.With(requireAuth(users, clock)).Get("/auth/session", wrapper.GetAuthSession)
		router.With(requireAuth(users, clock)).Delete("/auth/session", wrapper.DeleteAuthSession)
	})

	return router
}

func mustInvalidCredentialHash(hasher passwordHasher) string {
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

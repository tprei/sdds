package httpapi

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
	"net/http"
	"strings"
	"time"
)

type noteStores interface {
	note.Store
	note.AuthorNoteStore
}

type userStores interface {
	user.Store
	author.PublicAuthorStore
}

type server struct {
	notes                 note.Store
	catalog               note.Catalog
	users                 user.Store
	publicAuthors         author.PublicAuthorStore
	authorNotes           note.AuthorNoteStore
	passwordHasher        passwordHasher
	invalidCredentialHash string
	authRateLimiters      authRateLimiters
	newSessionToken       func() (string, error)
	clock                 func() time.Time
	readiness             ReadinessChecker
}

var _ openapi.ServerInterface = server{}

type passwordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, encoded string) (bool, error)
}

type ReadinessChecker interface {
	Check(context.Context) error
}

type AuthLimits struct {
	SignupRequestsPerMinute       int
	LoginRequestsPerMinute        int
	SignupGlobalRequestsPerMinute int
	LoginGlobalRequestsPerMinute  int
	PasswordHashConcurrency       int
}

func DefaultAuthLimits() AuthLimits {
	return AuthLimits{
		SignupRequestsPerMinute:       5,
		LoginRequestsPerMinute:        10,
		SignupGlobalRequestsPerMinute: 60,
		LoginGlobalRequestsPerMinute:  120,
		PasswordHashConcurrency:       2,
	}
}

func NewRouter(notes noteStores, catalog note.Catalog, users userStores, authLimits AuthLimits, readiness ReadinessChecker) http.Handler {
	hasher := newBoundedPasswordHasher(user.NewPasswordHasher(), authLimits.PasswordHashConcurrency)
	return newRouter(notes, catalog, users, hasher, mustInvalidCredentialHash(hasher), user.NewSessionToken, time.Now, authLimits, readiness)
}

func newRouter(
	notes noteStores,
	catalog note.Catalog,
	users userStores,
	passwordHasher passwordHasher,
	invalidCredentialHash string,
	newSessionToken func() (string, error),
	clock func() time.Time,
	authLimits AuthLimits,
	readiness ReadinessChecker,
) http.Handler {
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	validateOpenAPIRequest := openAPIRequestValidator()
	requireCurrentSession := requireAuth(users, clock)

	authRateLimiters := newAuthRateLimiters(authLimits, clock)
	handler := server{
		notes:                 notes,
		catalog:               catalog,
		users:                 users,
		publicAuthors:         users,
		authorNotes:           notes,
		passwordHasher:        passwordHasher,
		invalidCredentialHash: invalidCredentialHash,
		authRateLimiters:      authRateLimiters,
		newSessionToken:       newSessionToken,
		clock:                 clock,
		readiness:             readiness,
	}
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: writeGeneratedOpenAPIError,
	}

	router.With(validateOpenAPIRequest).Get("/healthz", wrapper.GetHealth)
	router.With(validateOpenAPIRequest).Get("/readyz", wrapper.GetReadiness)
	router.Route("/v1", func(router chi.Router) {
		router.Group(func(router chi.Router) {
			router.Use(validateOpenAPIRequest)
			router.Get("/categories", wrapper.ListCategories)
			router.Get("/places", wrapper.ListPlaces)
			router.Get("/notes", wrapper.ListNotes)
			router.Get("/authors/{author_id}", wrapper.GetAuthor)
			router.Get("/authors/{author_id}/notes", wrapper.ListAuthorNotes)
			router.Get("/notes/{note_id}", wrapper.GetNote)
			router.Get("/search/notes", wrapper.SearchNotes)
			router.Post("/auth/users", wrapper.CreateAuthUser)
			router.Post("/auth/sessions", wrapper.CreateAuthSession)
		})
		router.Group(func(router chi.Router) {
			router.Use(requireCurrentSession)
			router.Use(validateOpenAPIRequest)
			router.Post("/notes", wrapper.CreateNote)
			router.Get("/auth/session", wrapper.GetAuthSession)
			router.Delete("/auth/session", wrapper.DeleteAuthSession)
		})
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
		if response, ok := generatedInvalidAuthorNotesParamError(r.URL.Path, invalidParamError.ParamName); ok {
			writeError(w, http.StatusBadRequest, response)
			return
		}
		if code, ok := generatedInvalidParamErrorCode(r.URL.Path, invalidParamError.ParamName); ok {
			writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: code})
			return
		}
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func generatedInvalidAuthorNotesParamError(path string, paramName string) (openapi.ErrorResponse, bool) {
	if !strings.HasPrefix(path, "/v1/authors/") || !strings.HasSuffix(path, "/notes") {
		return openapi.ErrorResponse{}, false
	}
	if paramName != "limit" && paramName != "cursor" {
		return openapi.ErrorResponse{}, false
	}
	fields := []openapi.ValidationProblem{{
		Field: openapi.ValidationField(paramName),
		Code:  openapi.ValidationProblemCodeInvalid,
	}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidNote, Fields: &fields}, true
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

const readinessCheckTimeout = 2 * time.Second

func (handler server) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
	defer cancel()
	if handler.readiness == nil || handler.readiness.Check(ctx) != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	noContent(w, r)
}

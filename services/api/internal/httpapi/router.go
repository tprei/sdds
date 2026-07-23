package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type NoteStores interface {
	note.Store
	note.AuthorNoteStore
	note.UsefulStore
}

type UserStores interface {
	user.Store
	author.PublicAuthorStore
}

type ImageUploadPreparer interface {
	PrepareImageUpload(context.Context, string, media.UploadReceiver) (media.UploadReceipt, error)
}

type NotesDependencies struct {
	Stores  NoteStores
	Catalog note.Catalog
}

type AuthDependencies struct {
	Users  UserStores
	Limits AuthLimits
}

type MediaDependencies struct {
	ImageUploads   ImageUploadPreparer
	AttachedImages media.AttachedImageReader
}

type SystemDependencies struct {
	Readiness ReadinessChecker
}

type noteHandlers struct {
	store       note.Store
	authorNotes note.AuthorNoteStore
	useful      note.UsefulStore
	catalog     note.Catalog
}

type authHandlers struct {
	users                 user.Store
	publicAuthors         author.PublicAuthorStore
	passwordHasher        passwordHasher
	invalidCredentialHash string
	rateLimiters          authRateLimiters
	newSessionToken       func() (string, error)
	clock                 func() time.Time
}

type mediaHandlers struct {
	imageUploads         ImageUploadPreparer
	attachedImages       media.AttachedImageReader
	scratchDir           string
	responseWriteTimeout time.Duration
}

type systemHandlers struct {
	readiness ReadinessChecker
}

type server struct {
	notes  noteHandlers
	auth   authHandlers
	media  mediaHandlers
	system systemHandlers
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

func NewRouter(notes NotesDependencies, auth AuthDependencies, media MediaDependencies, system SystemDependencies) http.Handler {
	hasher := newBoundedPasswordHasher(user.NewPasswordHasher(), auth.Limits.PasswordHashConcurrency)
	return newRouter(
		noteHandlers{store: notes.Stores, authorNotes: notes.Stores, useful: notes.Stores, catalog: notes.Catalog},
		authHandlers{
			users:                 auth.Users,
			publicAuthors:         auth.Users,
			passwordHasher:        hasher,
			invalidCredentialHash: mustInvalidCredentialHash(hasher),
			rateLimiters:          newAuthRateLimiters(auth.Limits, time.Now),
			newSessionToken:       user.NewSessionToken,
			clock:                 time.Now,
		},
		mediaHandlers{imageUploads: media.ImageUploads, attachedImages: media.AttachedImages},
		systemHandlers{readiness: system.Readiness},
	)
}

func newRouter(notes noteHandlers, auth authHandlers, media mediaHandlers, system systemHandlers) http.Handler {
	if media.imageUploads == nil {
		panic("upload service is required")
	}
	if media.attachedImages == nil {
		panic("image reader is required")
	}
	router := chi.NewRouter()
	router.Use(localBrowserCORS)
	validateOpenAPIRequest := openAPIRequestValidator()
	requireCurrentSession := requireAuth(auth.users, auth.clock)
	handler := server{notes: notes, auth: auth, media: media, system: system}
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: writeGeneratedOpenAPIError,
	}

	router.With(validateOpenAPIRequest).Get("/healthz", wrapper.GetHealth)
	router.With(validateOpenAPIRequest).Get("/readyz", wrapper.GetReadiness)
	router.Route("/v1", func(router chi.Router) {
		router.Group(func(router chi.Router) {
			router.Use(validateOpenAPIRequest)
			router.Get("/media/images/{image_id}", wrapper.GetMediaImage)
			router.Post("/auth/users", wrapper.CreateAuthUser)
			router.Post("/auth/sessions", wrapper.CreateAuthSession)
		})
		router.Group(func(router chi.Router) {
			router.Use(requireCurrentSession)
			router.Use(validateOpenAPIRequest)
			router.Post("/media/image-uploads", wrapper.PrepareImageUpload)
		})
		router.Group(func(router chi.Router) {
			router.Use(requireCurrentSession)
			router.Use(validateOpenAPIRequest)
			router.Get("/categories", wrapper.ListCategories)
			router.Get("/places", wrapper.ListPlaces)
			router.Get("/notes", wrapper.ListNotes)
			router.Get("/authors/{author_id}", wrapper.GetAuthor)
			router.Get("/authors/{author_id}/notes", wrapper.ListAuthorNotes)
			router.Get("/notes/{note_id}", wrapper.GetNote)
			router.Get("/search/notes", wrapper.SearchNotes)
			router.Put("/notes/{note_id}/useful", wrapper.MarkNoteUseful)
			router.Delete("/notes/{note_id}/useful", wrapper.UnmarkNoteUseful)
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
	if handler.system.readiness == nil || handler.system.readiness.Check(ctx) != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	noContent(w, r)
}

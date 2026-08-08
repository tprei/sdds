package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

type fakeUploadPreparer struct {
	prepareImageUpload func(context.Context, string, media.UploadReceiver) (media.UploadReceipt, error)
}

func (fake fakeUploadPreparer) PrepareImageUpload(ctx context.Context, userID string, receive media.UploadReceiver) (media.UploadReceipt, error) {
	if fake.prepareImageUpload == nil {
		return media.UploadReceipt{}, errors.New("upload service not implemented")
	}
	return fake.prepareImageUpload(ctx, userID, receive)
}

type fakeReadiness struct {
	check func(context.Context) error
}

func (fake fakeReadiness) Check(ctx context.Context) error {
	if fake.check == nil {
		return nil
	}
	return fake.check(ctx)
}

type fakeCommentStore struct {
	createComment    func(context.Context, comment.CreateInput) (comment.Comment, error)
	findComment      func(context.Context, string, string) (comment.Comment, error)
	listNoteComments func(context.Context, comment.ListInput) (comment.Page, error)
	deleteComment    func(context.Context, string) error
	findCommentByID  func(context.Context, string) (comment.Comment, error)
	createReply      func(context.Context, comment.CreateReplyInput) (comment.Comment, error)
}

func (fake fakeCommentStore) CreateComment(ctx context.Context, input comment.CreateInput) (comment.Comment, error) {
	if fake.createComment == nil {
		return comment.Comment{}, errors.New("comment store not implemented")
	}
	return fake.createComment(ctx, input)
}

func (fake fakeCommentStore) FindComment(ctx context.Context, noteID string, commentID string) (comment.Comment, error) {
	if fake.findComment == nil {
		return comment.Comment{}, errors.New("comment store not implemented")
	}
	return fake.findComment(ctx, noteID, commentID)
}

func (fake fakeCommentStore) ListNoteComments(ctx context.Context, input comment.ListInput) (comment.Page, error) {
	if fake.listNoteComments == nil {
		return comment.Page{}, errors.New("comment store not implemented")
	}
	return fake.listNoteComments(ctx, input)
}

func (fake fakeCommentStore) DeleteComment(ctx context.Context, commentID string) error {
	if fake.deleteComment == nil {
		return errors.New("comment store not implemented")
	}
	return fake.deleteComment(ctx, commentID)
}

func (fake fakeCommentStore) FindCommentByID(ctx context.Context, id string) (comment.Comment, error) {
	if fake.findCommentByID == nil {
		return comment.Comment{}, errors.New("comment store not implemented")
	}
	return fake.findCommentByID(ctx, id)
}

func (fake fakeCommentStore) CreateReply(ctx context.Context, input comment.CreateReplyInput) (comment.Comment, error) {
	if fake.createReply == nil {
		return comment.Comment{}, errors.New("comment store not implemented")
	}
	return fake.createReply(ctx, input)
}

type fakeReportStore struct {
	createReport func(context.Context, report.CreateInput) (report.CreateResult, error)
}

func (fake fakeReportStore) CreateReport(ctx context.Context, input report.CreateInput) (report.CreateResult, error) {
	if fake.createReport == nil {
		return report.CreateResult{}, errors.New("report store not implemented")
	}
	return fake.createReport(ctx, input)
}

type fakeEventStore struct {
	appendBatch func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error)
}

func (fake fakeEventStore) AppendBatch(ctx context.Context, records []event.Record, receivedAt time.Time) (event.AppendBatchResult, error) {
	if fake.appendBatch == nil {
		return event.AppendBatchResult{}, errors.New("event store not implemented")
	}
	return fake.appendBatch(ctx, records, receivedAt)
}

func newRouterForTest(
	notes fakeNoteStore,
	catalog note.Catalog,
	users UserStores,
	authLimits AuthLimits,
	readiness ReadinessChecker,
	uploadService ImageUploadPreparer,
	imageReader media.AttachedImageReader,
) http.Handler {
	return NewRouter(
		NotesDependencies{Stores: notes, Publisher: notes, Searcher: notes, Catalog: catalog},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: users, ContactChannels: fakeContactChannelStore{}, Limits: authLimits},
		MediaDependencies{ImageUploads: uploadService, AttachedImages: imageReader},
		SystemDependencies{Readiness: readiness},
	)
}

func newRouterWithAuthSeamsForTest(
	notes fakeNoteStore,
	catalog note.Catalog,
	users UserStores,
	passwordHasher passwordHasher,
	invalidCredentialHash string,
	newSessionToken func() (string, error),
	clock func() time.Time,
	authLimits AuthLimits,
	readiness ReadinessChecker,
	uploadService ImageUploadPreparer,
	imageReader media.AttachedImageReader,
) http.Handler {
	return newRouter(
		noteHandlers{noteStore: notes, notePublisher: notes, noteSearcher: notes, authorNoteStore: notes, usefulStore: notes, categoryCatalog: catalog},
		commentHandlers{store: fakeCommentStore{}, notes: notes},
		reportHandlers{store: fakeReportStore{}, notes: notes, comments: fakeCommentStore{}},
		eventHandlers{store: fakeEventStore{}, limits: newEventRateLimiters(DefaultEventLimits(), clock), clock: clock},
		authHandlers{
			users:                 users,
			publicAuthors:         users,
			contactChannels:       fakeContactChannelStore{},
			passwordHasher:        passwordHasher,
			invalidCredentialHash: invalidCredentialHash,
			rateLimiters:          newAuthRateLimiters(authLimits, clock),
			newSessionToken:       newSessionToken,
			clock:                 clock,
		},
		mediaHandlers{imageUploads: uploadService, attachedImages: imageReader},
		systemHandlers{readiness: readiness},
	)
}

func TestNewRouterRequiresMediaDependencies(t *testing.T) {
	tests := []struct {
		name  string
		media MediaDependencies
	}{
		{name: "image upload preparer", media: MediaDependencies{AttachedImages: fakeAttachedImageReader{}}},
		{name: "attached image reader", media: MediaDependencies{ImageUploads: fakeUploadPreparer{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("NewRouter did not panic")
				}
			}()
			NewRouter(
				NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
				CommentDependencies{Store: fakeCommentStore{}},
				ReportDependencies{Store: fakeReportStore{}},
				EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
				AuthDependencies{Users: fakeUserStore{}, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
				test.media,
				SystemDependencies{Readiness: fakeReadiness{}},
			)
		})
	}
}

func TestNewRouterRequiresCommentStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewRouter did not panic")
		}
	}()
	NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{},
		ReportDependencies{Store: fakeReportStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: fakeUserStore{}, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	)
}

func TestNewRouterRequiresReportStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewRouter did not panic")
		}
	}()
	NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: fakeUserStore{}, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	)
}

func newTestRouter(notes fakeNoteStore) http.Handler {
	return withCurrentSessionHeader(newRouterForTest(
		notes,
		fakeCatalog{},
		authenticatedFakeUserStore(fakeUserStore{}),
		DefaultAuthLimits(),
		fakeReadiness{},
		fakeUploadPreparer{},
		fakeAttachedImageReader{},
	))
}

// withCurrentSessionHeader wraps a handler so every request carries the test
// bearer token required by the authenticated route group.
func withCurrentSessionHeader(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer current-token")
		handler.ServeHTTP(w, r)
	})
}

func TestHealthRoutesReturnNoContent(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "health", path: "/healthz"},
		{name: "ready", path: "/readyz"},
	}

	router := newTestRouter(fakeNoteStore{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
			if response.Body.Len() != 0 {
				t.Fatalf("body length = %d, want 0", response.Body.Len())
			}
		})
	}
}

func TestReadinessDegradesAndRecovers(t *testing.T) {
	available := true
	router := newRouterForTest(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{
		check: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("readiness context has no deadline")
			}
			if !available {
				return fmt.Errorf("dependency unavailable")
			}
			return nil
		},
	}, fakeUploadPreparer{}, fakeAttachedImageReader{})

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("ready status = %d, want %d", response.Code, http.StatusNoContent)
	}

	available = false
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("degraded ready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("degraded ready body length = %d, want 0", response.Body.Len())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("health status while degraded = %d, want %d", response.Code, http.StatusNoContent)
	}

	available = true
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("recovered ready status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestReadinessRejectsSentinelMismatch(t *testing.T) {
	router := newRouterForTest(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{
		check: func(context.Context) error {
			return media.ErrObjectIntegrity
		},
	}, fakeUploadPreparer{}, fakeAttachedImageReader{})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("sentinel mismatch status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestHealthRoutesRejectUnsupportedMethods(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	request := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterAllowsLocalBrowserOrigin(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(_ context.Context, _ note.ListInput) ([]note.Note, error) {
			return []note.Note{}, nil
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	gotHeaders := map[string]string{
		"Access-Control-Allow-Origin":  response.Header().Get("Access-Control-Allow-Origin"),
		"Access-Control-Allow-Methods": response.Header().Get("Access-Control-Allow-Methods"),
		"Access-Control-Allow-Headers": response.Header().Get("Access-Control-Allow-Headers"),
	}
	wantHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "http://localhost:8081",
		"Access-Control-Allow-Methods": corsAllowedMethods,
		"Access-Control-Allow-Headers": corsAllowedHeaders,
	}
	if diff := cmp.Diff(wantHeaders, gotHeaders); diff != "" {
		t.Fatalf("CORS headers mismatch (-want +got):\n%s", diff)
	}
}

func TestRouterRejectsNonLocalBrowserOrigin(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		listNotes: func(_ context.Context, _ note.ListInput) ([]note.Note, error) {
			return []note.Note{}, nil
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("access-control-allow-origin = %q, want empty", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRouterHandlesLocalBrowserPreflight(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	request := httptest.NewRequest(http.MethodOptions, "/v1/notes", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8081")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8081" {
		t.Fatalf("access-control-allow-origin = %q, want local origin", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRouterRejectsPlainOptionsRequest(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})
	request := httptest.NewRequest(http.MethodOptions, "/v1/notes", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

type fakeNoteStore struct {
	createNote      func(ctx context.Context, input note.CreateInput) (note.Note, error)
	publish         func(ctx context.Context, input note.CreateInput) (note.Note, error)
	findNote        func(ctx context.Context, id string, viewerUserID user.UserID) (note.Note, error)
	listNotes       func(ctx context.Context, input note.ListInput) ([]note.Note, error)
	searchNotes     func(ctx context.Context, input note.SearchInput) ([]note.Note, error)
	search          func(ctx context.Context, input note.SearchInput) ([]note.SearchResult, error)
	searchSemantic  func(ctx context.Context, input note.SemanticSearchInput) ([]note.ScoredNote, error)
	findNotesByID   func(ctx context.Context, ids []string, viewerUserID user.UserID) ([]note.Note, error)
	listAuthorNotes func(ctx context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error)
	markUseful      func(ctx context.Context, input note.MarkUsefulInput) error
	unmarkUseful    func(ctx context.Context, input note.UnmarkUsefulInput) error
}

func (store fakeNoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	if store.createNote == nil {
		return note.Note{}, fmt.Errorf("create note not implemented")
	}
	return store.createNote(ctx, input)
}

func (store fakeNoteStore) Publish(ctx context.Context, input note.CreateInput) (note.Note, error) {
	if store.publish == nil {
		return note.Note{}, fmt.Errorf("publish not implemented")
	}
	return store.publish(ctx, input)
}

func (store fakeNoteStore) FindNote(ctx context.Context, id string, viewerUserID user.UserID) (note.Note, error) {
	if store.findNote == nil {
		return note.Note{}, fmt.Errorf("find note not implemented")
	}
	return store.findNote(ctx, id, viewerUserID)
}

func (store fakeNoteStore) ListRecentNotes(ctx context.Context, input note.ListInput) ([]note.Note, error) {
	if store.listNotes == nil {
		return nil, fmt.Errorf("list notes not implemented")
	}
	return store.listNotes(ctx, input)
}

func (store fakeNoteStore) SearchNotes(ctx context.Context, input note.SearchInput) ([]note.Note, error) {
	if store.searchNotes == nil {
		return nil, fmt.Errorf("search notes not implemented")
	}
	return store.searchNotes(ctx, input)
}

// Search is the NoteSearcher seam. It defaults to wrapping SearchNotes
// results as lexical so every existing test that only sets searchNotes
// keeps working unchanged; tests exercising hybrid-specific behavior (e.g.
// per-result retrieval sources, embedder failure) set search directly.
func (store fakeNoteStore) Search(ctx context.Context, input note.SearchInput) ([]note.SearchResult, error) {
	if store.search != nil {
		return store.search(ctx, input)
	}
	notes, err := store.SearchNotes(ctx, input)
	if err != nil {
		return nil, err
	}
	results := make([]note.SearchResult, len(notes))
	for i, found := range notes {
		results[i] = note.SearchResult{Note: found, RetrievalSource: note.RetrievalSourceLexical}
	}
	return results, nil
}

func (store fakeNoteStore) SearchSemantic(ctx context.Context, input note.SemanticSearchInput) ([]note.ScoredNote, error) {
	if store.searchSemantic == nil {
		return nil, fmt.Errorf("search semantic not implemented")
	}
	return store.searchSemantic(ctx, input)
}

func (store fakeNoteStore) FindNotesByID(ctx context.Context, ids []string, viewerUserID user.UserID) ([]note.Note, error) {
	if store.findNotesByID == nil {
		return nil, fmt.Errorf("find notes by id not implemented")
	}
	return store.findNotesByID(ctx, ids, viewerUserID)
}

func (store fakeNoteStore) ListAuthorNotes(ctx context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error) {
	if store.listAuthorNotes == nil {
		return note.AuthorNotesPage{}, fmt.Errorf("list author notes not implemented")
	}
	return store.listAuthorNotes(ctx, input)
}

func (store fakeNoteStore) MarkUseful(ctx context.Context, input note.MarkUsefulInput) error {
	if store.markUseful == nil {
		return fmt.Errorf("mark useful not implemented")
	}
	return store.markUseful(ctx, input)
}

func (store fakeNoteStore) UnmarkUseful(ctx context.Context, input note.UnmarkUsefulInput) error {
	if store.unmarkUseful == nil {
		return fmt.Errorf("unmark useful not implemented")
	}
	return store.unmarkUseful(ctx, input)
}

type fakeCatalog struct {
	listCategories     func(ctx context.Context) ([]note.Category, error)
	findActiveCategory func(ctx context.Context, slug note.CategorySlug) (note.Category, error)
}

func (catalog fakeCatalog) ListCategories(ctx context.Context) ([]note.Category, error) {
	if catalog.listCategories != nil {
		return catalog.listCategories(ctx)
	}
	return note.Categories, nil
}

func (catalog fakeCatalog) FindActiveCategory(ctx context.Context, slug note.CategorySlug) (note.Category, error) {
	if catalog.findActiveCategory != nil {
		return catalog.findActiveCategory(ctx, slug)
	}
	for _, category := range note.Categories {
		if category.Slug == slug && category.Active {
			return category, nil
		}
	}
	return note.Category{}, note.ErrCategoryNotFound
}

type fakeUserStore struct {
	createPasswordUser func(ctx context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error)
	findPasswordLogin  func(ctx context.Context, normalizedUsername string) (user.PasswordLogin, error)
	createSession      func(ctx context.Context, input user.CreateSessionInput) (user.CurrentSession, error)
	findCurrentSession func(ctx context.Context, tokenHash string, now time.Time) (user.CurrentSession, error)
	revokeSession      func(ctx context.Context, sessionID user.SessionID, revokedAt time.Time) error
	findAuthorByUserID func(ctx context.Context, userID user.UserID) (user.Author, error)
	deleteUser         func(ctx context.Context, userID user.UserID, deletedAt time.Time) error
	findPublicAuthor   func(ctx context.Context, authorID author.AuthorID) (author.PublicAuthor, error)
}

func (store fakeUserStore) CreatePasswordUser(ctx context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
	if store.createPasswordUser == nil {
		return user.CurrentSession{}, fmt.Errorf("create password user not implemented")
	}
	return store.createPasswordUser(ctx, input)
}

func (store fakeUserStore) FindPasswordLogin(ctx context.Context, normalizedUsername string) (user.PasswordLogin, error) {
	if store.findPasswordLogin == nil {
		return user.PasswordLogin{}, fmt.Errorf("find password login not implemented")
	}
	return store.findPasswordLogin(ctx, normalizedUsername)
}

func (store fakeUserStore) CreateSession(ctx context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
	if store.createSession == nil {
		return user.CurrentSession{}, fmt.Errorf("create session not implemented")
	}
	return store.createSession(ctx, input)
}

func (store fakeUserStore) FindCurrentSession(ctx context.Context, tokenHash string, now time.Time) (user.CurrentSession, error) {
	if store.findCurrentSession == nil {
		return user.CurrentSession{}, fmt.Errorf("find current session not implemented")
	}
	return store.findCurrentSession(ctx, tokenHash, now)
}

// testCurrentUserSessionResolver is the explicit session resolver used by
// authenticated test routers. It accepts the "current-token" bearer and
// resolves the fixed test identity used across handler tests.
func testCurrentUserSessionResolver(_ context.Context, tokenHash string, _ time.Time) (user.CurrentSession, error) {
	if tokenHash != user.HashSessionToken("current-token") {
		return user.CurrentSession{}, user.ErrSessionNotFound
	}
	return user.CurrentSession{
		Session: user.Session{UserID: "user-id-thiago", TokenHash: tokenHash},
		User:    user.User{ID: "user-id-thiago", State: user.UserStateActive},
		Author:  user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
	}, nil
}

// authenticatedFakeUserStore returns a fakeUserStore whose findCurrentSession
// resolves the test bearer token, preserving any overrides supplied by the
// caller (for example a custom findPublicAuthor).
func authenticatedFakeUserStore(overrides fakeUserStore) fakeUserStore {
	overrides.findCurrentSession = testCurrentUserSessionResolver
	return overrides
}

func (store fakeUserStore) RevokeSession(ctx context.Context, sessionID user.SessionID, revokedAt time.Time) error {
	if store.revokeSession == nil {
		return fmt.Errorf("revoke session not implemented")
	}
	return store.revokeSession(ctx, sessionID, revokedAt)
}

func (store fakeUserStore) FindAuthorByUserID(ctx context.Context, userID user.UserID) (user.Author, error) {
	if store.findAuthorByUserID == nil {
		return user.Author{}, fmt.Errorf("find author by user id not implemented")
	}
	return store.findAuthorByUserID(ctx, userID)
}

func (store fakeUserStore) DeleteUser(ctx context.Context, userID user.UserID, deletedAt time.Time) error {
	if store.deleteUser == nil {
		return fmt.Errorf("delete user not implemented")
	}
	return store.deleteUser(ctx, userID, deletedAt)
}

func (store fakeUserStore) FindPublicAuthor(ctx context.Context, authorID author.AuthorID) (author.PublicAuthor, error) {
	if store.findPublicAuthor == nil {
		return author.PublicAuthor{}, fmt.Errorf("find public author not implemented")
	}
	return store.findPublicAuthor(ctx, authorID)
}

type fakeContactChannelStore struct {
	upsertUnverifiedEmail       func(ctx context.Context, userID user.UserID, value string, now time.Time) (user.ContactChannelRecord, error)
	findEmailForUser            func(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error)
	findPendingEmailForUser     func(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error)
	findVerifiedEmail           func(ctx context.Context, value string) (user.ContactChannelRecord, error)
	createToken                 func(ctx context.Context, input user.CreateContactChannelTokenInput) error
	consumeTokenAndMarkVerified func(ctx context.Context, tokenHash string, verifiedVia string, now time.Time) (user.ContactChannelRecord, error)
	consumeTokenAndSetPassword  func(ctx context.Context, tokenHash string, secretHash string, now time.Time) (user.ContactChannelRecord, error)
}

func (store fakeContactChannelStore) UpsertUnverifiedEmail(ctx context.Context, userID user.UserID, value string, now time.Time) (user.ContactChannelRecord, error) {
	if store.upsertUnverifiedEmail == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return store.upsertUnverifiedEmail(ctx, userID, value, now)
}

func (store fakeContactChannelStore) FindEmailForUser(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error) {
	if store.findEmailForUser == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return store.findEmailForUser(ctx, userID)
}

func (store fakeContactChannelStore) FindPendingEmailForUser(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error) {
	if store.findPendingEmailForUser == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return store.findPendingEmailForUser(ctx, userID)
}

func (store fakeContactChannelStore) FindVerifiedEmail(ctx context.Context, value string) (user.ContactChannelRecord, error) {
	if store.findVerifiedEmail == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return store.findVerifiedEmail(ctx, value)
}

func (store fakeContactChannelStore) CreateToken(ctx context.Context, input user.CreateContactChannelTokenInput) error {
	if store.createToken == nil {
		return nil
	}
	return store.createToken(ctx, input)
}

func (store fakeContactChannelStore) ConsumeTokenAndMarkVerified(ctx context.Context, tokenHash string, verifiedVia string, now time.Time) (user.ContactChannelRecord, error) {
	if store.consumeTokenAndMarkVerified == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}
	return store.consumeTokenAndMarkVerified(ctx, tokenHash, verifiedVia, now)
}

func (store fakeContactChannelStore) ConsumeTokenAndSetPassword(ctx context.Context, tokenHash string, secretHash string, now time.Time) (user.ContactChannelRecord, error) {
	if store.consumeTokenAndSetPassword == nil {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}
	return store.consumeTokenAndSetPassword(ctx, tokenHash, secretHash, now)
}

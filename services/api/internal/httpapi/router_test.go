package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

type fakeReadiness struct {
	check func(context.Context) error
}

func (fake fakeReadiness) Check(ctx context.Context) error {
	if fake.check == nil {
		return nil
	}
	return fake.check(ctx)
}

func newTestRouter(notes fakeNoteStore) http.Handler {
	handler := NewRouter(notes, fakeCatalog{}, fakeUserStore{
		findCurrentSession: func(_ context.Context, tokenHash string, _ time.Time) (user.CurrentSession, error) {
			if tokenHash != user.HashSessionToken("current-token") {
				return user.CurrentSession{}, user.ErrSessionNotFound
			}
			return user.CurrentSession{
				Session: user.Session{UserID: "user-id-thiago", TokenHash: tokenHash},
				User:    user.User{ID: "user-id-thiago", State: user.UserStateActive},
				Author:  user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
			}, nil
		},
	}, DefaultAuthLimits(), fakeReadiness{})
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
	router := NewRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{
		check: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("readiness context has no deadline")
			}
			if !available {
				return fmt.Errorf("dependency unavailable")
			}
			return nil
		},
	})

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
	router := NewRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{
		check: func(context.Context) error {
			return media.ErrObjectIntegrity
		},
	})

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
	findNote        func(ctx context.Context, id string) (note.Note, error)
	listNotes       func(ctx context.Context, input note.ListInput) ([]note.Note, error)
	searchNotes     func(ctx context.Context, input note.SearchInput) ([]note.Note, error)
	listAuthorNotes func(ctx context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error)
}

func (store fakeNoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	if store.createNote == nil {
		return note.Note{}, fmt.Errorf("create note not implemented")
	}
	return store.createNote(ctx, input)
}

func (store fakeNoteStore) FindNote(ctx context.Context, id string) (note.Note, error) {
	if store.findNote == nil {
		return note.Note{}, fmt.Errorf("find note not implemented")
	}
	return store.findNote(ctx, id)
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

func (store fakeNoteStore) ListAuthorNotes(ctx context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error) {
	if store.listAuthorNotes == nil {
		return note.AuthorNotesPage{}, fmt.Errorf("list author notes not implemented")
	}
	return store.listAuthorNotes(ctx, input)
}

type fakeCatalog struct {
	listCategories     func(ctx context.Context) ([]note.Category, error)
	listPlaces         func(ctx context.Context) ([]note.Place, error)
	findActiveCategory func(ctx context.Context, slug note.CategorySlug) (note.Category, error)
	findActivePlace    func(ctx context.Context, slug note.PlaceSlug) (note.Place, error)
}

func (catalog fakeCatalog) ListCategories(ctx context.Context) ([]note.Category, error) {
	if catalog.listCategories != nil {
		return catalog.listCategories(ctx)
	}
	return note.Categories, nil
}

func (catalog fakeCatalog) ListPlaces(ctx context.Context) ([]note.Place, error) {
	if catalog.listPlaces != nil {
		return catalog.listPlaces(ctx)
	}
	return note.Places, nil
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

func (catalog fakeCatalog) FindActivePlace(ctx context.Context, slug note.PlaceSlug) (note.Place, error) {
	if catalog.findActivePlace != nil {
		return catalog.findActivePlace(ctx, slug)
	}
	for _, place := range note.Places {
		if place.Slug == slug && place.Active {
			return place, nil
		}
	}
	return note.Place{}, note.ErrPlaceNotFound
}

type fakeUserStore struct {
	createPasswordUser func(ctx context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error)
	findPasswordLogin  func(ctx context.Context, normalizedUsername string) (user.PasswordLogin, error)
	createSession      func(ctx context.Context, input user.CreateSessionInput) (user.CurrentSession, error)
	findCurrentSession func(ctx context.Context, tokenHash string, now time.Time) (user.CurrentSession, error)
	revokeSession      func(ctx context.Context, sessionID user.SessionID, revokedAt time.Time) error
	findAuthorByUserID func(ctx context.Context, userID user.UserID) (user.Author, error)
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

func (store fakeUserStore) FindPublicAuthor(ctx context.Context, authorID author.AuthorID) (author.PublicAuthor, error) {
	if store.findPublicAuthor == nil {
		return author.PublicAuthor{}, fmt.Errorf("find public author not implemented")
	}
	return store.findPublicAuthor(ctx, authorID)
}

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type usefulCallLog struct {
	mu    sync.Mutex
	calls int
}

func newUsefulCallLog() *usefulCallLog { return &usefulCallLog{} }

func (log *usefulCallLog) mark(input note.MarkUsefulInput, t *testing.T) func(context.Context, note.MarkUsefulInput) error {
	t.Helper()
	return func(_ context.Context, actual note.MarkUsefulInput) error {
		log.mu.Lock()
		defer log.mu.Unlock()
		if actual != input {
			t.Fatalf("mark input = %#v, want %#v", actual, input)
		}
		log.calls++
		return nil
	}
}

func (log *usefulCallLog) unmark(input note.UnmarkUsefulInput, t *testing.T) func(context.Context, note.UnmarkUsefulInput) error {
	t.Helper()
	return func(_ context.Context, actual note.UnmarkUsefulInput) error {
		log.mu.Lock()
		defer log.mu.Unlock()
		if actual != input {
			t.Fatalf("unmark input = %#v, want %#v", actual, input)
		}
		log.calls++
		return nil
	}
}

func (log *usefulCallLog) assertCount(t *testing.T, want int) {
	t.Helper()
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.calls != want {
		t.Fatalf("useful calls = %d, want %d", log.calls, want)
	}
}

func TestMarkNoteUsefulReturnsNoContent(t *testing.T) {
	markCalls := newUsefulCallLog()
	router := newTestRouter(fakeNoteStore{
		findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) {
			if id != exampleNoteID {
				t.Fatalf("find id = %q, want %q", id, exampleNoteID)
			}
			return note.Note{ID: id}, nil
		},
		markUseful: markCalls.mark(note.MarkUsefulInput{NoteID: exampleNoteID, UserID: "user-id-thiago"}, t),
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty", response.Body.String())
	}
	markCalls.assertCount(t, 1)
}

func TestMarkNoteUsefulIsIdempotent(t *testing.T) {
	markCalls := newUsefulCallLog()
	router := newTestRouter(fakeNoteStore{
		findNote:   func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		markUseful: markCalls.mark(note.MarkUsefulInput{NoteID: exampleNoteID, UserID: "user-id-thiago"}, t),
	})

	for attempt := 1; attempt <= 2; attempt++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/notes/"+exampleNoteID+"/useful", nil)
		router.ServeHTTP(response, request)
		requireOpenAPIResponse(t, request, response)
		if response.Code != http.StatusNoContent {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, http.StatusNoContent)
		}
		if response.Body.Len() != 0 {
			t.Fatalf("attempt %d body = %q, want empty", attempt, response.Body.String())
		}
	}
	markCalls.assertCount(t, 2)
}

func TestMarkNoteUsefulReturnsNotFoundForUnknownNote(t *testing.T) {
	markCalled := false
	router := newTestRouter(fakeNoteStore{
		findNote: func(context.Context, string, user.UserID) (note.Note, error) {
			return note.Note{}, note.ErrNoteNotFound
		},
		markUseful: func(_ context.Context, _ note.MarkUsefulInput) error {
			markCalled = true
			return nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes/missing/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
	if markCalled {
		t.Fatal("mark useful was called for an unknown note")
	}
}

func TestMarkNoteUsefulReturnsInternalErrorOnStoreFailure(t *testing.T) {
	router := newTestRouter(fakeNoteStore{
		findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		markUseful: func(context.Context, note.MarkUsefulInput) error {
			return errors.New("database unavailable")
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	requireErrorCode(t, response, openapi.ErrorCodeInternal)
}

func TestMarkNoteUsefulRejectsMissingSessionBeforeValidation(t *testing.T) {
	markCalled := false
	router := newRouterForTest(fakeNoteStore{
		findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		markUseful: func(_ context.Context, _ note.MarkUsefulInput) error {
			markCalled = true
			return nil
		},
	}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
	if markCalled {
		t.Fatal("mark useful was called without a session")
	}
}

func TestUnmarkNoteUsefulReturnsNoContent(t *testing.T) {
	unmarkCalls := newUsefulCallLog()
	router := newTestRouter(fakeNoteStore{
		findNote:     func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		unmarkUseful: unmarkCalls.unmark(note.UnmarkUsefulInput{NoteID: exampleNoteID, UserID: "user-id-thiago"}, t),
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty", response.Body.String())
	}
	unmarkCalls.assertCount(t, 1)
}

func TestUnmarkNoteUsefulIsIdempotentWithoutPriorMark(t *testing.T) {
	unmarkCalls := newUsefulCallLog()
	router := newTestRouter(fakeNoteStore{
		findNote:     func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		unmarkUseful: unmarkCalls.unmark(note.UnmarkUsefulInput{NoteID: exampleNoteID, UserID: "user-id-thiago"}, t),
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %q, want empty", response.Body.String())
	}
	unmarkCalls.assertCount(t, 1)
}

func TestUnmarkNoteUsefulReturnsNotFoundForUnknownNote(t *testing.T) {
	unmarkCalled := false
	router := newTestRouter(fakeNoteStore{
		findNote: func(context.Context, string, user.UserID) (note.Note, error) {
			return note.Note{}, note.ErrNoteNotFound
		},
		unmarkUseful: func(_ context.Context, _ note.UnmarkUsefulInput) error {
			unmarkCalled = true
			return nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/missing/useful", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
	if unmarkCalled {
		t.Fatal("unmark useful was called for an unknown note")
	}
}

func TestUnmarkNoteUsefulRejectsMissingSessionBeforeValidation(t *testing.T) {
	unmarkCalled := false
	router := newRouterForTest(fakeNoteStore{
		findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) { return note.Note{ID: id}, nil },
		unmarkUseful: func(_ context.Context, _ note.UnmarkUsefulInput) error {
			unmarkCalled = true
			return nil
		},
	}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/useful", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
	if unmarkCalled {
		t.Fatal("unmark useful was called without a session")
	}
}

func TestUsefulEndpointsAllowLocalBrowserPutPreflight(t *testing.T) {
	router := newTestRouter(fakeNoteStore{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/v1/notes/"+exampleNoteID+"/useful", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8081")
	request.Header.Set("Access-Control-Request-Method", http.MethodPut)
	request.Header.Set("Access-Control-Request-Headers", "Authorization")

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != corsAllowedHeaders {
		t.Fatalf("allow headers = %q, want %q", got, corsAllowedHeaders)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != corsAllowedMethods {
		t.Fatalf("allow methods = %q, want %q", got, corsAllowedMethods)
	}
}

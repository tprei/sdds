package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestHealthRoutesReturnNoContent(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "health", path: "/healthz"},
		{name: "ready", path: "/readyz"},
	}

	router := NewRouter(fakeNoteStore{})

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

func TestHealthRoutesRejectUnsupportedMethods(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	request := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterAllowsLocalBrowserOrigin(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		listNotes: func(_ context.Context, _ int) ([]note.Note, error) {
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
	router := NewRouter(fakeNoteStore{
		listNotes: func(_ context.Context, _ int) ([]note.Note, error) {
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
	router := NewRouter(fakeNoteStore{})
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
	router := NewRouter(fakeNoteStore{})
	request := httptest.NewRequest(http.MethodOptions, "/v1/notes", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

type fakeNoteStore struct {
	createNote func(ctx context.Context, input note.CreateInput) (note.Note, error)
	listNotes  func(ctx context.Context, limit int) ([]note.Note, error)
}

func (store fakeNoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	if store.createNote == nil {
		return note.Note{}, fmt.Errorf("create note not implemented")
	}
	return store.createNote(ctx, input)
}

func (store fakeNoteStore) ListRecentNotes(ctx context.Context, limit int) ([]note.Note, error) {
	if store.listNotes == nil {
		return nil, fmt.Errorf("list notes not implemented")
	}
	return store.listNotes(ctx, limit)
}

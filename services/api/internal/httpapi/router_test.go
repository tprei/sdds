package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

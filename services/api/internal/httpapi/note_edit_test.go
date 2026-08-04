package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type fakeNoteEditor struct {
	edit   func(context.Context, note.EditInput) (note.Note, error)
	delete func(context.Context, string, user.UserID) error
}

func (fake fakeNoteEditor) Edit(ctx context.Context, input note.EditInput) (note.Note, error) {
	if fake.edit == nil {
		return note.Note{}, errors.New("editor Edit not implemented")
	}
	return fake.edit(ctx, input)
}

func (fake fakeNoteEditor) Delete(ctx context.Context, noteID string, userID user.UserID) error {
	if fake.delete == nil {
		return errors.New("editor Delete not implemented")
	}
	return fake.delete(ctx, noteID, userID)
}

func newNoteEditTestRouter(editor fakeNoteEditor) http.Handler {
	return withCurrentSessionHeader(NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}, Editor: editor},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: authenticatedFakeUserStore(fakeUserStore{}), Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	))
}

func doNoteEditRequest(t *testing.T, router http.Handler, method, path string, body []byte) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	response := httptest.NewRecorder()
	var request *http.Request
	if body != nil {
		request = httptest.NewRequest(method, path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	} else {
		request = httptest.NewRequest(method, path, nil)
	}
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	return response, request
}

func decodeErrorResponse(t *testing.T, response *httptest.ResponseRecorder) openapi.ErrorResponse {
	t.Helper()
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return body
}

func TestUpdateNoteReturnsUpdatedNote(t *testing.T) {
	router := newNoteEditTestRouter(fakeNoteEditor{
		edit: func(_ context.Context, input note.EditInput) (note.Note, error) {
			if input.NoteID != exampleNoteID {
				t.Fatalf("note id = %q, want %q", input.NoteID, exampleNoteID)
			}
			if input.Title == nil || *input.Title != "Título novo" {
				t.Fatalf("title = %v, want %q", input.Title, "Título novo")
			}
			return note.Note{ID: exampleNoteID, Title: "Título novo", Body: "Corpo", CategorySlug: note.CategorySlugFood}, nil
		},
	})

	body := []byte(`{"title":"Título novo"}`)
	response, _ := doNoteEditRequest(t, router, http.MethodPatch, "/v1/notes/"+exampleNoteID, body)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var noteBody openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &noteBody); err != nil {
		t.Fatalf("decode note response: %v", err)
	}
	if noteBody.Title != "Título novo" {
		t.Fatalf("title = %q, want %q", noteBody.Title, "Título novo")
	}
}

func TestUpdateNoteMapsValidationToInvalidNote(t *testing.T) {
	router := newNoteEditTestRouter(fakeNoteEditor{
		edit: func(context.Context, note.EditInput) (note.Note, error) {
			return note.Note{}, &note.EditValidationError{Problems: []note.ValidationProblem{{Field: "title", Message: "too_short"}}}
		},
	})

	response, _ := doNoteEditRequest(t, router, http.MethodPatch, "/v1/notes/"+exampleNoteID, []byte(`{"title":"ab"}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	body := decodeErrorResponse(t, response)
	if body.Code != openapi.ErrorCodeInvalidNote {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
	}
	if body.Fields == nil || len(*body.Fields) != 1 || (*body.Fields)[0].Field != "title" {
		t.Fatalf("fields = %v, want one title problem", body.Fields)
	}
}

func TestUpdateNoteMapsErrors(t *testing.T) {
	tests := []struct {
		name    string
		editErr error
		status  int
		code    openapi.ErrorCode
	}{
		{name: "forbidden", editErr: note.ErrNoteForbidden, status: http.StatusForbidden, code: openapi.ErrorCodeForbidden},
		{name: "not found", editErr: note.ErrNoteNotFound, status: http.StatusNotFound, code: openapi.ErrorCodeNotFound},
		{name: "embedding unavailable", editErr: note.ErrEmbeddingUnavailable, status: http.StatusServiceUnavailable, code: openapi.ErrorCodeEmbeddingUnavailable},
		{name: "internal", editErr: errors.New("database unavailable"), status: http.StatusInternalServerError, code: openapi.ErrorCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newNoteEditTestRouter(fakeNoteEditor{
				edit: func(context.Context, note.EditInput) (note.Note, error) {
					return note.Note{}, tt.editErr
				},
			})
			response, _ := doNoteEditRequest(t, router, http.MethodPatch, "/v1/notes/"+exampleNoteID, []byte(`{"body":"novo corpo"}`))
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d", response.Code, tt.status)
			}
			if body := decodeErrorResponse(t, response); body.Code != tt.code {
				t.Fatalf("code = %s, want %s", body.Code, tt.code)
			}
		})
	}
}

func TestDeleteNoteReturnsNoContent(t *testing.T) {
	router := newNoteEditTestRouter(fakeNoteEditor{
		delete: func(_ context.Context, noteID string, _ user.UserID) error {
			if noteID != exampleNoteID {
				t.Fatalf("note id = %q, want %q", noteID, exampleNoteID)
			}
			return nil
		},
	})
	response, _ := doNoteEditRequest(t, router, http.MethodDelete, "/v1/notes/"+exampleNoteID, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestDeleteNoteMapsErrors(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		status    int
		code      openapi.ErrorCode
	}{
		{name: "forbidden", deleteErr: note.ErrNoteForbidden, status: http.StatusForbidden, code: openapi.ErrorCodeForbidden},
		{name: "not found", deleteErr: note.ErrNoteNotFound, status: http.StatusNotFound, code: openapi.ErrorCodeNotFound},
		{name: "internal", deleteErr: errors.New("database unavailable"), status: http.StatusInternalServerError, code: openapi.ErrorCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newNoteEditTestRouter(fakeNoteEditor{
				delete: func(context.Context, string, user.UserID) error { return tt.deleteErr },
			})
			response, _ := doNoteEditRequest(t, router, http.MethodDelete, "/v1/notes/"+exampleNoteID, nil)
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d", response.Code, tt.status)
			}
			if body := decodeErrorResponse(t, response); body.Code != tt.code {
				t.Fatalf("code = %s, want %s", body.Code, tt.code)
			}
		})
	}
}

func TestUpdateNoteEnforcesExactRequestSizeLimit(t *testing.T) {
	validBody := append([]byte(`{"title":"x"}`), bytes.Repeat([]byte(" "), int(maxUpdateNoteRequestBytes)-len(`{"title":"x"}`))...)
	if int64(len(validBody)) != maxUpdateNoteRequestBytes {
		t.Fatalf("valid body length = %d, want %d", len(validBody), maxUpdateNoteRequestBytes)
	}

	editCalls := 0
	router := newNoteEditTestRouter(fakeNoteEditor{
		edit: func(_ context.Context, _ note.EditInput) (note.Note, error) {
			editCalls++
			return note.Note{ID: exampleNoteID, Title: "x", Body: "body", CategorySlug: note.CategorySlugFood}, nil
		},
	})

	response, _ := doNoteEditRequest(t, router, http.MethodPatch, "/v1/notes/"+exampleNoteID, validBody)
	if response.Code != http.StatusOK {
		t.Fatalf("exact boundary status = %d, want %d", response.Code, http.StatusOK)
	}
	if editCalls != 1 {
		t.Fatalf("edit calls = %d, want 1", editCalls)
	}

	oversized := append(validBody, ' ')
	response, _ = doNoteEditRequest(t, router, http.MethodPatch, "/v1/notes/"+exampleNoteID, oversized)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	requireErrorCode(t, response, openapi.ErrorCodeRequestTooLarge)
	if editCalls != 1 {
		t.Fatalf("edit calls after oversized request = %d, want 1", editCalls)
	}
}

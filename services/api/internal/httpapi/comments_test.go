package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/pagination"
	"github.com/tprei/sdds/services/api/internal/user"
)

const exampleCommentID = "comment-id-1"

func TestCreateNoteCommentReturnsPublicComment(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 123_000_000, time.UTC)
	created := false
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		createComment: func(_ context.Context, input comment.CreateInput) (comment.Comment, error) {
			created = true
			want := comment.CreateInput{NoteID: exampleNoteID, UserID: "user-id-thiago", Body: "Comentário útil"}
			if input != want {
				t.Fatalf("create input = %#v, want %#v", input, want)
			}
			return comment.Comment{
				ID:        exampleCommentID,
				NoteID:    input.NoteID,
				UserID:    input.UserID,
				Body:      input.Body,
				Author:    comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
				CreatedAt: createdAt,
			}, nil
		},
	})
	request := jsonRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", `{"body":"  Comentário útil  "}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !created {
		t.Fatal("CreateComment was not called")
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wire, "id", "body", "author", "created_at", "parent_comment_id")
	if wire["parent_comment_id"] != nil {
		t.Fatalf("parent_comment_id = %#v, want null for top-level comment", wire["parent_comment_id"])
	}
	if wire["id"] != exampleCommentID {
		t.Fatalf("id = %#v, want %q", wire["id"], exampleCommentID)
	}
	if wire["body"] != "Comentário útil" {
		t.Fatalf("body = %#v, want trimmed body", wire["body"])
	}
	requireJSONNumber(t, wire, "created_at", createdAt.UnixMilli())
	authorWire, ok := wire["author"].(map[string]any)
	if !ok {
		t.Fatalf("author = %T, want object", wire["author"])
	}
	requireExactJSONKeys(t, authorWire, "id", "display_name")
	if authorWire["id"] != "author-id-thiago" || authorWire["display_name"] != "Thiago" {
		t.Fatalf("author = %#v, want public author summary", authorWire)
	}
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestCreateNoteCommentAccepts1000UnicodeCodePoints(t *testing.T) {
	body := strings.Repeat("😀", comment.BodyMaxLength)
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		createComment: func(_ context.Context, input comment.CreateInput) (comment.Comment, error) {
			if input.Body != body {
				t.Fatalf("body = %q, want 1000 code points", input.Body)
			}
			return comment.Comment{
				ID:        exampleCommentID,
				NoteID:    input.NoteID,
				UserID:    input.UserID,
				Body:      input.Body,
				Author:    comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
				CreatedAt: time.Now(),
			}, nil
		},
	})
	request := jsonRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", `{"body":"`+body+`"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
}

func TestListNoteCommentsDefaultsLimitAndReturnsOpaqueCursor(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		listNoteComments: func(_ context.Context, input comment.ListInput) (comment.Page, error) {
			if input.NoteID != exampleNoteID {
				t.Fatalf("note ID = %q, want %q", input.NoteID, exampleNoteID)
			}
			if input.Limit != comment.ListDefaultLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, comment.ListDefaultLimit)
			}
			if input.After != nil {
				t.Fatalf("after = %#v, want nil", input.After)
			}
			return comment.Page{
				Comments: []comment.ListedComment{{
					Comment: comment.Comment{
						ID:        exampleCommentID,
						NoteID:    exampleNoteID,
						UserID:    "user-id-thiago",
						Body:      "Muito bom",
						Author:    comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
						CreatedAt: createdAt,
					},
					Position: comment.Position{PageKey: 8},
				}},
				HasMore: true,
			}, nil
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/"+exampleNoteID+"/comments", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wire, "threads", "next_cursor")
	threadsWire, ok := wire["threads"].([]any)
	if !ok || len(threadsWire) != 1 {
		t.Fatalf("threads = %#v, want one thread", wire["threads"])
	}
	threadWire, ok := threadsWire[0].(map[string]any)
	if !ok {
		t.Fatalf("thread = %T, want object", threadsWire[0])
	}
	requireExactJSONKeys(t, threadWire, "comment", "replies", "has_more_replies")
	commentWire, ok := threadWire["comment"].(map[string]any)
	if !ok {
		t.Fatalf("comment = %T, want object", threadWire["comment"])
	}
	requireExactJSONKeys(t, commentWire, "id", "body", "author", "created_at", "parent_comment_id")
	nextCursor, ok := wire["next_cursor"].(string)
	if !ok || nextCursor == "" {
		t.Fatalf("next_cursor = %#v, want non-empty string", wire["next_cursor"])
	}
	var cursor commentCursorPayload
	if err := pagination.Decode(nextCursor, &cursor); err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	if cursor != (commentCursorPayload{Version: 1, PageKey: 8}) {
		t.Fatalf("cursor = %#v, want %#v", cursor, commentCursorPayload{Version: 1, PageKey: 8})
	}
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestListNoteCommentsPassesContinuationCursor(t *testing.T) {
	encoded, err := encodeCommentCursor(comment.Position{PageKey: 4})
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		listNoteComments: func(_ context.Context, input comment.ListInput) (comment.Page, error) {
			if input.Limit != 1 {
				t.Fatalf("limit = %d, want 1", input.Limit)
			}
			if input.After == nil || input.After.PageKey != 4 {
				t.Fatalf("after = %#v, want page key 4", input.After)
			}
			return comment.Page{Comments: []comment.ListedComment{}, HasMore: false}, nil
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/notes/"+exampleNoteID+"/comments?limit=1&cursor="+encoded, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	if wire["next_cursor"] != nil {
		t.Fatalf("next_cursor = %#v, want null", wire["next_cursor"])
	}
	threadsWire, ok := wire["threads"].([]any)
	if !ok || len(threadsWire) != 0 {
		t.Fatalf("threads = %#v, want empty array", wire["threads"])
	}
}

func TestCreateNoteCommentRejectsInvalidBodies(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCode   openapi.ErrorCode
		wantFields []openapi.ValidationProblem
	}{
		{
			name:       "missing body",
			body:       `{}`,
			wantCode:   openapi.ErrorCodeInvalidComment,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "whitespace body",
			body:       `{"body":" \t\n "}`,
			wantCode:   openapi.ErrorCodeInvalidComment,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "1001 Unicode code points",
			body:       `{"body":"` + strings.Repeat("😀", comment.BodyMaxLength+1) + `"}`,
			wantCode:   openapi.ErrorCodeInvalidComment,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeTooLong}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
				createComment: func(context.Context, comment.CreateInput) (comment.Comment, error) {
					t.Fatal("CreateComment should not be called")
					return comment.Comment{}, nil
				},
			})
			request := jsonRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			var body openapi.ErrorResponse
			if err := decodeJSONResponse(response, &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != test.wantCode {
				t.Fatalf("code = %s, want %s", body.Code, test.wantCode)
			}
			requireValidationProblems(t, body.Fields, test.wantFields)
		})
	}
}

func TestCreateNoteCommentRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"body":`},
		{name: "unknown field", body: `{"body":"ok","extra":true}`},
		{name: "wrong body type", body: `{"body":42}`},
		{name: "trailing JSON", body: `{"body":"ok"} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
				createComment: func(context.Context, comment.CreateInput) (comment.Comment, error) {
					t.Fatal("CreateComment should not be called")
					return comment.Comment{}, nil
				},
			})
			request := jsonRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			requireErrorCode(t, response, openapi.ErrorCodeInvalidJSON)
		})
	}
}

func TestCreateNoteCommentEnforcesExactRequestSizeLimit(t *testing.T) {
	validBody := append([]byte(`{"body":"ok"}`), bytes.Repeat([]byte(" "), int(maxCreateCommentRequestBytes)-len(`{"body":"ok"}`))...)
	if int64(len(validBody)) != maxCreateCommentRequestBytes {
		t.Fatalf("valid body length = %d, want %d", len(validBody), maxCreateCommentRequestBytes)
	}
	created := 0
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		createComment: func(_ context.Context, input comment.CreateInput) (comment.Comment, error) {
			created++
			return comment.Comment{
				ID:        exampleCommentID,
				NoteID:    input.NoteID,
				UserID:    input.UserID,
				Body:      input.Body,
				Author:    comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
				CreatedAt: time.Now(),
			}, nil
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", bytes.NewReader(validBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("exact boundary status = %d, want %d", response.Code, http.StatusCreated)
	}
	if created != 1 {
		t.Fatalf("create calls = %d, want 1", created)
	}

	oversized := append(validBody, ' ')
	request = httptest.NewRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", bytes.NewReader(oversized))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	requireErrorCode(t, response, openapi.ErrorCodeRequestTooLarge)
	if created != 1 {
		t.Fatalf("create calls after oversized request = %d, want 1", created)
	}
}

func TestListNoteCommentsRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field openapi.ValidationField
	}{
		{name: "zero limit", query: "?limit=0", field: openapi.ValidationFieldLimit},
		{name: "oversized limit", query: "?limit=51", field: openapi.ValidationFieldLimit},
		{name: "empty cursor", query: "?cursor=", field: openapi.ValidationFieldCursor},
		{name: "malformed cursor", query: "?cursor=not-a-cursor", field: openapi.ValidationFieldCursor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
				listNoteComments: func(context.Context, comment.ListInput) (comment.Page, error) {
					t.Fatal("ListNoteComments should not be called")
					return comment.Page{}, nil
				},
			})
			request := httptest.NewRequest(http.MethodGet, "/v1/notes/"+exampleNoteID+"/comments"+test.query, nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			var body openapi.ErrorResponse
			if err := decodeJSONResponse(response, &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidComment {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidComment)
			}
			requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{{Field: test.field, Code: openapi.ValidationProblemCodeInvalid}})
		})
	}
}

func TestCommentRoutesDoNotCallCommentStoreForMissingNote(t *testing.T) {
	methods := []struct {
		name string
		path string
		body string
	}{
		{name: "list", path: "/v1/notes/missing/comments"},
		{name: "create", path: "/v1/notes/missing/comments", body: `{"body":"ok"}`},
		{name: "delete", path: "/v1/notes/missing/comments/" + exampleCommentID},
	}
	for _, test := range methods {
		t.Run(test.name, func(t *testing.T) {
			notes := fakeNoteStore{findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) {
				if id != "missing" {
					t.Fatalf("note ID = %q, want missing", id)
				}
				return note.Note{}, note.ErrNoteNotFound
			}}
			router := newCommentRouter(notes, fakeCommentStore{
				createComment: func(context.Context, comment.CreateInput) (comment.Comment, error) {
					t.Fatal("CreateComment should not be called")
					return comment.Comment{}, nil
				},
				findComment: func(context.Context, string, string) (comment.Comment, error) {
					t.Fatal("FindComment should not be called")
					return comment.Comment{}, nil
				},
				listNoteComments: func(context.Context, comment.ListInput) (comment.Page, error) {
					t.Fatal("ListNoteComments should not be called")
					return comment.Page{}, nil
				},
				deleteComment: func(context.Context, string) error { t.Fatal("DeleteComment should not be called"); return nil },
			})
			request := jsonRequest(http.MethodGet, test.path, test.body)
			if test.name == "create" {
				request = jsonRequest(http.MethodPost, test.path, test.body)
			}
			if test.name == "delete" {
				request = httptest.NewRequest(http.MethodDelete, test.path, nil)
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			requireErrorCode(t, response, openapi.ErrorCodeNotFound)
		})
	}
}

func TestDeleteNoteCommentRequiresOwner(t *testing.T) {
	deleted := false
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		findComment: func(_ context.Context, noteID string, commentID string) (comment.Comment, error) {
			if noteID != exampleNoteID || commentID != exampleCommentID {
				t.Fatalf("find scope = (%q, %q), want (%q, %q)", noteID, commentID, exampleNoteID, exampleCommentID)
			}
			return comment.Comment{ID: exampleCommentID, NoteID: noteID, UserID: "other-user"}, nil
		},
		deleteComment: func(context.Context, string) error {
			deleted = true
			return nil
		},
	})
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/comments/"+exampleCommentID, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	requireErrorCode(t, response, openapi.ErrorCodeForbidden)
	if deleted {
		t.Fatal("DeleteComment should not be called for a non-owner")
	}
}

func TestDeleteNoteCommentDeletesOwnedComment(t *testing.T) {
	deletedID := ""
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		findComment: func(context.Context, string, string) (comment.Comment, error) {
			return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID, UserID: "user-id-thiago"}, nil
		},
		deleteComment: func(_ context.Context, commentID string) error {
			deletedID = commentID
			return nil
		},
	})
	request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/comments/"+exampleCommentID, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", response.Body.Len())
	}
	if deletedID != exampleCommentID {
		t.Fatalf("deleted ID = %q, want %q", deletedID, exampleCommentID)
	}
}

func TestDeleteNoteCommentMapsMissingAndStoreFailures(t *testing.T) {
	tests := []struct {
		name       string
		store      fakeCommentStore
		wantStatus int
		wantCode   openapi.ErrorCode
	}{
		{
			name: "missing comment",
			store: fakeCommentStore{findComment: func(context.Context, string, string) (comment.Comment, error) {
				return comment.Comment{}, comment.ErrCommentNotFound
			}},
			wantStatus: http.StatusNotFound,
			wantCode:   openapi.ErrorCodeNotFound,
		},
		{
			name: "find failure",
			store: fakeCommentStore{findComment: func(context.Context, string, string) (comment.Comment, error) {
				return comment.Comment{}, errors.New("database unavailable")
			}},
			wantStatus: http.StatusInternalServerError,
			wantCode:   openapi.ErrorCodeInternal,
		},
		{
			name: "delete race",
			store: fakeCommentStore{
				findComment: func(context.Context, string, string) (comment.Comment, error) {
					return comment.Comment{ID: exampleCommentID, UserID: "user-id-thiago"}, nil
				},
				deleteComment: func(context.Context, string) error { return comment.ErrCommentNotFound },
			},
			wantStatus: http.StatusNotFound,
			wantCode:   openapi.ErrorCodeNotFound,
		},
		{
			name: "delete failure",
			store: fakeCommentStore{
				findComment: func(context.Context, string, string) (comment.Comment, error) {
					return comment.Comment{ID: exampleCommentID, UserID: "user-id-thiago"}, nil
				},
				deleteComment: func(context.Context, string) error { return errors.New("database unavailable") },
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   openapi.ErrorCodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), test.store)
			request := httptest.NewRequest(http.MethodDelete, "/v1/notes/"+exampleNoteID+"/comments/"+exampleCommentID, nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			requireErrorCode(t, response, test.wantCode)
		})
	}
}

func TestCommentRoutesMapStoreFailures(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		store  fakeCommentStore
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/v1/notes/" + exampleNoteID + "/comments",
			body:   `{"body":"select * from note_comments"}`,
			store: fakeCommentStore{createComment: func(context.Context, comment.CreateInput) (comment.Comment, error) {
				return comment.Comment{}, errors.New("database unavailable")
			}},
		},
		{
			name:   "list",
			method: http.MethodGet,
			path:   "/v1/notes/" + exampleNoteID + "/comments",
			store: fakeCommentStore{listNoteComments: func(context.Context, comment.ListInput) (comment.Page, error) {
				return comment.Page{}, errors.New("database unavailable")
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), test.store)
			request := jsonRequest(test.method, test.path, test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
			}
			requireErrorCode(t, response, openapi.ErrorCodeInternal)
		})
	}
}

func TestCreateNoteCommentAuthenticatesBeforeReadingBody(t *testing.T) {
	body := &countingReader{Reader: strings.NewReader(`{"body":`)}
	router := newRouterForTest(
		fakeNoteStore{},
		fakeCatalog{},
		fakeUserStore{findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrSessionNotFound
		}},
		DefaultAuthLimits(),
		fakeReadiness{},
		fakeUploadPreparer{},
		fakeAttachedImageReader{},
	)
	request := httptest.NewRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestCommentRoutesRejectUnauthenticatedSessions(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		storeError    error
	}{
		{name: "missing header"},
		{name: "malformed header", authorization: "Token abc"},
		{name: "unknown token", authorization: "Bearer missing", storeError: user.ErrSessionNotFound},
		{name: "expired", authorization: "Bearer expired", storeError: user.ErrSessionExpired},
		{name: "revoked", authorization: "Bearer revoked", storeError: user.ErrSessionRevoked},
		{name: "disabled user", authorization: "Bearer disabled", storeError: user.ErrUserDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Catalog: fakeCatalog{}},
				CommentDependencies{Store: fakeCommentStore{}},
				ReportDependencies{Store: fakeReportStore{}},
				EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
				AuthDependencies{Users: fakeUserStore{
					findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
						if test.storeError == nil {
							t.Fatal("FindCurrentSession should not be called")
						}
						return user.CurrentSession{}, test.storeError
					},
				}, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
				MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
				SystemDependencies{Readiness: fakeReadiness{}},
				PublicReadDependencies{},
			)
			request := jsonRequest(http.MethodPost, "/v1/notes/"+exampleNoteID+"/comments", `{"body":"ok"}`)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
		})
	}
}

func newCommentRouter(notes fakeNoteStore, comments fakeCommentStore) http.Handler {
	return withCurrentSessionHeader(NewRouter(
		NotesDependencies{Stores: notes, Publisher: notes, Catalog: fakeCatalog{}},
		CommentDependencies{Store: comments},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: comments},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: authenticatedFakeUserStore(fakeUserStore{}), ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
		PublicReadDependencies{},
	))
}

func commentFoundNoteStore() fakeNoteStore {
	return fakeNoteStore{findNote: func(_ context.Context, id string, viewerUserID user.UserID) (note.Note, error) {
		if id != exampleNoteID {
			return note.Note{}, note.ErrNoteNotFound
		}
		if viewerUserID != "user-id-thiago" {
			return note.Note{}, errors.New("unexpected viewer")
		}
		return note.Note{ID: id}, nil
	}}
}

func decodeJSONResponse(response *httptest.ResponseRecorder, target any) error {
	return json.Unmarshal(response.Body.Bytes(), target)
}

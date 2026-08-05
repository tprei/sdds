// Handler tests for comment replies: creating a reply under a top-level
// comment and listing comments as one-level threads.

package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestCreateCommentReplyReturnsPublicReply(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 123_000_000, time.UTC)
	created := false
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		findCommentByID: func(_ context.Context, id string) (comment.Comment, error) {
			if id != exampleCommentID {
				t.Fatalf("find by id = %q, want %q", id, exampleCommentID)
			}
			return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID, UserID: "user-id-other"}, nil
		},
		createReply: func(_ context.Context, input comment.CreateReplyInput) (comment.Comment, error) {
			created = true
			want := comment.CreateReplyInput{ParentCommentID: comment.CommentID(exampleCommentID), UserID: "user-id-thiago", Body: "Resposta útil"}
			if input != want {
				t.Fatalf("create reply input = %#v, want %#v", input, want)
			}
			return comment.Comment{
				ID:              "reply-id-1",
				NoteID:          exampleNoteID,
				UserID:          input.UserID,
				Body:            input.Body,
				Author:          comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
				CreatedAt:       createdAt,
				ParentCommentID: comment.CommentID(exampleCommentID),
			}, nil
		},
	})
	request := jsonRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", `{"body":"  Resposta útil  "}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !created {
		t.Fatal("CreateReply was not called")
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wire, "id", "body", "author", "created_at", "parent_comment_id")
	if wire["id"] != "reply-id-1" {
		t.Fatalf("id = %#v, want reply-id-1", wire["id"])
	}
	if wire["body"] != "Resposta útil" {
		t.Fatalf("body = %#v, want trimmed body", wire["body"])
	}
	parentCommentID, ok := wire["parent_comment_id"].(string)
	if !ok || parentCommentID != exampleCommentID {
		t.Fatalf("parent_comment_id = %#v, want %q", wire["parent_comment_id"], exampleCommentID)
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

func TestCreateCommentReplyRequiresSession(t *testing.T) {
	router := NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: fakeUserStore{
			findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
				return user.CurrentSession{}, user.ErrSessionNotFound
			},
		}, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	)
	request := jsonRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", `{"body":"ok"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
}

func TestCreateCommentReplyRejectsInvalidBodies(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantFields []openapi.ValidationProblem
	}{
		{
			name:       "missing body",
			body:       `{}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "whitespace body",
			body:       `{"body":" \t\n "}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "1001 Unicode code points",
			body:       `{"body":"` + strings.Repeat("😀", comment.BodyMaxLength+1) + `"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: openapi.ValidationProblemCodeTooLong}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
				findCommentByID: func(_ context.Context, id string) (comment.Comment, error) {
					return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID}, nil
				},
				createReply: func(context.Context, comment.CreateReplyInput) (comment.Comment, error) {
					t.Fatal("CreateReply should not be called")
					return comment.Comment{}, nil
				},
			})
			request := jsonRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", test.body)
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
			requireValidationProblems(t, body.Fields, test.wantFields)
		})
	}
}

func TestCreateCommentReplyEnforcesExactRequestSizeLimit(t *testing.T) {
	validBody := append([]byte(`{"body":"ok"}`), bytes.Repeat([]byte(" "), int(maxCreateCommentRequestBytes)-len(`{"body":"ok"}`))...)
	if int64(len(validBody)) != maxCreateCommentRequestBytes {
		t.Fatalf("valid body length = %d, want %d", len(validBody), maxCreateCommentRequestBytes)
	}
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 123_000_000, time.UTC)
	created := 0
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		findCommentByID: func(_ context.Context, id string) (comment.Comment, error) {
			return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID}, nil
		},
		createReply: func(_ context.Context, input comment.CreateReplyInput) (comment.Comment, error) {
			created++
			return comment.Comment{
				ID:              "reply-id",
				NoteID:          exampleNoteID,
				UserID:          input.UserID,
				Body:            input.Body,
				Author:          comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
				CreatedAt:       createdAt,
				ParentCommentID: input.ParentCommentID,
			}, nil
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", bytes.NewReader(validBody))
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
	request = httptest.NewRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", bytes.NewReader(oversized))
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

func TestCreateCommentReplyMapsParentAndStoreFailures(t *testing.T) {
	topLevelParent := func(_ context.Context, id string) (comment.Comment, error) {
		return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID}, nil
	}
	tests := []struct {
		name       string
		store      fakeCommentStore
		wantStatus int
		wantCode   openapi.ErrorCode
	}{
		{
			name: "unknown parent",
			store: fakeCommentStore{findCommentByID: func(context.Context, string) (comment.Comment, error) {
				return comment.Comment{}, comment.ErrCommentNotFound
			}},
			wantStatus: http.StatusNotFound,
			wantCode:   openapi.ErrorCodeNotFound,
		},
		{
			name: "parent is reply",
			store: fakeCommentStore{findCommentByID: func(context.Context, string) (comment.Comment, error) {
				return comment.Comment{ID: exampleCommentID, NoteID: exampleNoteID, ParentCommentID: "other-parent"}, nil
			}},
			wantStatus: http.StatusConflict,
			wantCode:   openapi.ErrorCodeInvalidReplyTarget,
		},
		{
			name: "find failure",
			store: fakeCommentStore{findCommentByID: func(context.Context, string) (comment.Comment, error) {
				return comment.Comment{}, errors.New("database unavailable")
			}},
			wantStatus: http.StatusInternalServerError,
			wantCode:   openapi.ErrorCodeInternal,
		},
		{
			name: "create race not top level",
			store: fakeCommentStore{
				findCommentByID: topLevelParent,
				createReply: func(context.Context, comment.CreateReplyInput) (comment.Comment, error) {
					return comment.Comment{}, comment.ErrParentCommentNotTopLevel
				},
			},
			wantStatus: http.StatusConflict,
			wantCode:   openapi.ErrorCodeInvalidReplyTarget,
		},
		{
			name: "create missing parent race",
			store: fakeCommentStore{
				findCommentByID: topLevelParent,
				createReply: func(context.Context, comment.CreateReplyInput) (comment.Comment, error) {
					return comment.Comment{}, comment.ErrCommentNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   openapi.ErrorCodeNotFound,
		},
		{
			name: "create failure",
			store: fakeCommentStore{
				findCommentByID: topLevelParent,
				createReply: func(context.Context, comment.CreateReplyInput) (comment.Comment, error) {
					return comment.Comment{}, errors.New("database unavailable")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   openapi.ErrorCodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newCommentRouter(commentFoundNoteStore(), test.store)
			request := jsonRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", `{"body":"Resposta"}`)
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

func TestCreateCommentReplyRejectsInvisibleParentNote(t *testing.T) {
	notes := fakeNoteStore{findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) {
		if id != "secret-note" {
			t.Fatalf("note ID = %q, want secret-note", id)
		}
		return note.Note{}, note.ErrNoteNotFound
	}}
	router := newCommentRouter(notes, fakeCommentStore{
		findCommentByID: func(_ context.Context, id string) (comment.Comment, error) {
			return comment.Comment{ID: exampleCommentID, NoteID: "secret-note"}, nil
		},
		createReply: func(context.Context, comment.CreateReplyInput) (comment.Comment, error) {
			t.Fatal("CreateReply should not be called")
			return comment.Comment{}, nil
		},
	})
	request := jsonRequest(http.MethodPost, "/v1/comments/"+exampleCommentID+"/replies", `{"body":"Resposta"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
}

func TestListNoteCommentsReturnsThreads(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	router := newCommentRouter(commentFoundNoteStore(), fakeCommentStore{
		listNoteComments: func(_ context.Context, input comment.ListInput) (comment.Page, error) {
			if input.NoteID != exampleNoteID {
				t.Fatalf("note ID = %q, want %q", input.NoteID, exampleNoteID)
			}
			return comment.Page{
				Comments: []comment.ListedComment{{
					Comment: comment.Comment{
						ID:        exampleCommentID,
						NoteID:    exampleNoteID,
						UserID:    "user-id-thiago",
						Body:      "Comentário principal",
						Author:    comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
						CreatedAt: createdAt,
					},
					Position: comment.Position{PageKey: 8},
					Replies: []comment.Comment{{
						ID:              "reply-id-1",
						NoteID:          exampleNoteID,
						UserID:          "user-id-thiago",
						Body:            "Primeira resposta",
						Author:          comment.AuthorSummary{ID: "author-id-thiago", DisplayName: "Thiago"},
						CreatedAt:       createdAt.Add(time.Minute),
						ParentCommentID: comment.CommentID(exampleCommentID),
					}},
					HasMoreReplies: true,
				}},
				HasMore: false,
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
	if wire["next_cursor"] != nil {
		t.Fatalf("next_cursor = %#v, want null", wire["next_cursor"])
	}
	threadsWire, ok := wire["threads"].([]any)
	if !ok || len(threadsWire) != 1 {
		t.Fatalf("threads = %#v, want one thread", wire["threads"])
	}
	threadWire, ok := threadsWire[0].(map[string]any)
	if !ok {
		t.Fatalf("thread = %T, want object", threadsWire[0])
	}
	requireExactJSONKeys(t, threadWire, "comment", "replies", "has_more_replies")
	if threadWire["has_more_replies"] != true {
		t.Fatalf("has_more_replies = %#v, want true", threadWire["has_more_replies"])
	}
	parentWire, ok := threadWire["comment"].(map[string]any)
	if !ok {
		t.Fatalf("comment = %T, want object", threadWire["comment"])
	}
	requireExactJSONKeys(t, parentWire, "id", "body", "author", "created_at", "parent_comment_id")
	if parentWire["id"] != exampleCommentID {
		t.Fatalf("comment id = %#v, want %q", parentWire["id"], exampleCommentID)
	}
	if parentWire["parent_comment_id"] != nil {
		t.Fatalf("parent comment parent_comment_id = %#v, want null", parentWire["parent_comment_id"])
	}
	repliesWire, ok := threadWire["replies"].([]any)
	if !ok || len(repliesWire) != 1 {
		t.Fatalf("replies = %#v, want one reply", threadWire["replies"])
	}
	replyWire, ok := repliesWire[0].(map[string]any)
	if !ok {
		t.Fatalf("reply = %T, want object", repliesWire[0])
	}
	requireExactJSONKeys(t, replyWire, "id", "body", "author", "created_at", "parent_comment_id")
	if replyWire["id"] != "reply-id-1" {
		t.Fatalf("reply id = %#v, want reply-id-1", replyWire["id"])
	}
	if replyWire["body"] != "Primeira resposta" {
		t.Fatalf("reply body = %#v, want Primeira resposta", replyWire["body"])
	}
	if replyWire["parent_comment_id"] != exampleCommentID {
		t.Fatalf("reply parent_comment_id = %#v, want %q", replyWire["parent_comment_id"], exampleCommentID)
	}
	requireNoPrivateWireFields(t, response.Body.String())
}

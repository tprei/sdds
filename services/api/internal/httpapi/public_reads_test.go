package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const publicReadAuthorID = "author-id-thiago"

// publicReadTestRouter builds an unwrapped router (no Authorization header)
// seeded with a single note the viewer has marked useful, so authenticated
// responses carry useful_by_current_user=true and anonymous responses omit it.
func publicReadTestRouter(t *testing.T) http.Handler {
	t.Helper()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	markedNote := note.Note{
		ID:                  exampleNoteID,
		Title:               "Café bom",
		Body:                "Tem pão de queijo decente.",
		CategorySlug:        note.CategorySlugFood,
		Author:              note.AuthorSummary{ID: publicReadAuthorID, DisplayName: "Thiago"},
		UsefulByCurrentUser: true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	store := fakeNoteStore{
		findNote: func(context.Context, string, user.UserID) (note.Note, error) {
			return markedNote, nil
		},
		listNotes: func(context.Context, note.ListInput) ([]note.Note, error) {
			return []note.Note{markedNote}, nil
		},
		searchNotes: func(context.Context, note.SearchInput) ([]note.Note, error) {
			return []note.Note{markedNote}, nil
		},
		listAuthorNotes: func(context.Context, note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			return note.AuthorNotesPage{Notes: []note.AuthorNote{{Note: markedNote}}}, nil
		},
	}
	users := authenticatedFakeUserStore(fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: publicReadAuthorID, DisplayName: "Thiago", NoteCount: 1}, nil
		},
	})
	comments := fakeCommentStore{
		listNoteComments: func(context.Context, comment.ListInput) (comment.Page, error) {
			return comment.Page{Comments: []comment.ListedComment{{Comment: comment.Comment{ID: "comment-1", Body: "Boa!"}}}}, nil
		},
	}
	return NewRouter(
		NotesDependencies{Stores: store, Publisher: store, Searcher: store, Catalog: fakeCatalog{}},
		CommentDependencies{Store: comments},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: comments},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: users, ContactChannels: fakeContactChannelStore{}, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
		PublicReadDependencies{Limits: DefaultPublicReadLimits()},
	)
}

func doRequest(t *testing.T, router http.Handler, method, path, authHeader string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if authHeader != "" {
		request.Header.Set("Authorization", authHeader)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// noteRecords pulls every note object out of a decoded response, covering the
// single-note detail, the list/search envelopes, and the author-notes page.
func noteRecords(body map[string]any) []map[string]any {
	if note, ok := body["useful_count"]; ok {
		_ = note
		return []map[string]any{body}
	}
	if notes, ok := body["notes"].([]any); ok {
		return maps(notes)
	}
	if results, ok := body["results"].([]any); ok {
		out := make([]map[string]any, 0, len(results))
		for _, result := range results {
			if record, ok := result.(map[string]any); ok {
				if inner, ok := record["note"].(map[string]any); ok {
					out = append(out, inner)
				}
			}
		}
		return out
	}
	return nil
}

func maps(values []any) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if record, ok := value.(map[string]any); ok {
			out = append(out, record)
		}
	}
	return out
}

func assertNoViewerField(t *testing.T, path string, record map[string]any) {
	t.Helper()
	if _, ok := record["useful_by_current_user"]; ok {
		t.Fatalf("%s: anonymous response must omit useful_by_current_user", path)
	}
}

func TestPublicReadsAnswerAnonymousCallers(t *testing.T) {
	router := publicReadTestRouter(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "categories", path: "/v1/categories"},
		{name: "notes", path: "/v1/notes"},
		{name: "note", path: "/v1/notes/" + exampleNoteID},
		{name: "note comments", path: "/v1/notes/" + exampleNoteID + "/comments"},
		{name: "author", path: "/v1/authors/" + publicReadAuthorID},
		{name: "author notes", path: "/v1/authors/" + publicReadAuthorID + "/notes"},
		{name: "search", path: "/v1/search/notes?q=caf%C3%A9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := doRequest(t, router, http.MethodGet, test.path, "", "")

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusOK, response.Body.String())
			}
		})
	}
}

func TestAnonymousReadsOmitViewerField(t *testing.T) {
	router := publicReadTestRouter(t)

	paths := []string{
		"/v1/notes/" + exampleNoteID,
		"/v1/notes",
		"/v1/search/notes?q=caf%C3%A9",
		"/v1/authors/" + publicReadAuthorID + "/notes",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			records := noteRecords(decodeResponseObject(t, doRequest(t, router, http.MethodGet, path, "", "").Body.Bytes()))
			if len(records) == 0 {
				t.Fatalf("%s: no notes in response body", path)
			}
			for _, record := range records {
				assertNoViewerField(t, path, record)
			}
		})
	}
}

func TestAuthenticatedReadsIncludeViewerField(t *testing.T) {
	router := publicReadTestRouter(t)

	paths := []string{
		"/v1/notes/" + exampleNoteID,
		"/v1/notes",
		"/v1/search/notes?q=caf%C3%A9",
		"/v1/authors/" + publicReadAuthorID + "/notes",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			records := noteRecords(decodeResponseObject(t, doRequest(t, router, http.MethodGet, path, "Bearer current-token", "").Body.Bytes()))
			if len(records) == 0 {
				t.Fatalf("%s: no notes in response body", path)
			}
			for _, record := range records {
				value, ok := record["useful_by_current_user"]
				if !ok {
					t.Fatalf("%s: expected useful_by_current_user for authenticated caller", path)
				}
				if value != true {
					t.Fatalf("%s: useful_by_current_user = %v, want true", path, value)
				}
			}
		})
	}
}

func TestPresentButInvalidTokenStillRejected(t *testing.T) {
	router := publicReadTestRouter(t)

	response := doRequest(t, router, http.MethodGet, "/v1/notes", "Bearer not-a-real-token", "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestWritesStillRequireAuthentication(t *testing.T) {
	router := publicReadTestRouter(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create note", method: http.MethodPost, path: "/v1/notes", body: `{"title":"x","body":"y","category_slug":"food"}`},
		{name: "update note", method: http.MethodPatch, path: "/v1/notes/" + exampleNoteID, body: `{"title":"x"}`},
		{name: "delete note", method: http.MethodDelete, path: "/v1/notes/" + exampleNoteID},
		{name: "create comment", method: http.MethodPost, path: "/v1/notes/" + exampleNoteID + "/comments", body: `{"body":"x"}`},
		{name: "delete comment", method: http.MethodDelete, path: "/v1/notes/" + exampleNoteID + "/comments/comment-1"},
		{name: "create reply", method: http.MethodPost, path: "/v1/comments/comment-1/replies", body: `{"body":"x"}`},
		{name: "mark useful", method: http.MethodPut, path: "/v1/notes/" + exampleNoteID + "/useful"},
		{name: "unmark useful", method: http.MethodDelete, path: "/v1/notes/" + exampleNoteID + "/useful"},
		{name: "report", method: http.MethodPost, path: "/v1/reports", body: `{"target_type":"note","target_id":"x","reason":"spam"}`},
		{name: "events", method: http.MethodPost, path: "/v1/events", body: `{"events":[]}`},
		{name: "image upload", method: http.MethodPost, path: "/v1/media/image-uploads", body: `{"content_type":"image/jpeg"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := doRequest(t, router, test.method, test.path, "", test.body)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusUnauthorized, response.Body.String())
			}
			body := decodeResponseObject(t, response.Body.Bytes())
			if body["code"] != "unauthenticated" {
				t.Fatalf("error code = %v, want unauthenticated", body["code"])
			}
		})
	}
}

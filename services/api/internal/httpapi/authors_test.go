package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const (
	exampleAuthorID        = author.AuthorID("018ff5b8-0000-7000-8000-000000000010")
	exampleAuthorNoteID    = "018ff5b8-0000-7000-8000-000000000011"
	exampleCursorPageKey   = int64(123)
	exampleAuthorDisplay   = "Marina Alves"
	exampleAuthorCreatedMS = int64(1782993600000)
)

func TestGetAuthorReturnsPublicProfileWithoutAuthentication(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(_ context.Context, authorID author.AuthorID) (author.PublicAuthor, error) {
			if authorID != exampleAuthorID {
				t.Fatalf("author id = %q, want %q", authorID, exampleAuthorID)
			}
			return author.PublicAuthor{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay, NoteCount: 27}, nil
		},
	}, DefaultAuthLimits())

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID), nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.PublicAuthor
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.PublicAuthor{Id: string(exampleAuthorID), DisplayName: exampleAuthorDisplay, NoteCount: 27}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response body mismatch (-want +got):\n%s", diff)
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wireBody, "id", "display_name", "note_count")
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestGetAuthorReturnsNotFound(t *testing.T) {
	router := NewRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{}, author.ErrAuthorNotFound
		},
	}, DefaultAuthLimits())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/missing-author", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
}

func TestListAuthorNotesDefaultsLimitAndReturnsOpaqueCursor(t *testing.T) {
	createdAt := time.UnixMilli(exampleAuthorCreatedMS).UTC()
	router := NewRouter(fakeNoteStore{
		listAuthorNotes: func(_ context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			if input.AuthorID != exampleAuthorID {
				t.Fatalf("author id = %q, want %q", input.AuthorID, exampleAuthorID)
			}
			if input.Limit != note.AuthorNotesDefaultLimit {
				t.Fatalf("limit = %d, want %d", input.Limit, note.AuthorNotesDefaultLimit)
			}
			if input.After != nil {
				t.Fatalf("cursor = %#v, want nil", input.After)
			}
			return note.AuthorNotesPage{Notes: []note.AuthorNote{authorHTTPAuthorNote(exampleAuthorNoteID, createdAt, exampleCursorPageKey)}, HasMore: true}, nil
		},
	}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay, NoteCount: 2}, nil
		},
	}, DefaultAuthLimits())

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID)+"/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.AuthorNotesPage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Notes) != 1 {
		t.Fatalf("note count = %d, want 1", len(body.Notes))
	}
	if body.NextCursor == nil {
		t.Fatal("next_cursor = nil, want cursor")
	}
	cursor, problems := decodeAuthorNotesCursor(body.NextCursor)
	if len(problems) > 0 {
		t.Fatalf("returned cursor did not decode: %#v", problems)
	}
	if cursor == nil || cursor.PageKey != exampleCursorPageKey || !cursor.CreatedAt.Equal(createdAt) {
		t.Fatalf("decoded cursor = %#v, want final returned note position", cursor)
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wireBody, "notes", "next_cursor")
	notesValue, ok := wireBody["notes"].([]any)
	if !ok || len(notesValue) != 1 {
		t.Fatalf("wire notes = %#v, want one note", wireBody["notes"])
	}
	noteValue, ok := notesValue[0].(map[string]any)
	if !ok {
		t.Fatalf("wire note = %T, want object", notesValue[0])
	}
	requireExactJSONKeys(t, noteValue, "id", "title", "body", "category_slug", "place_slug", "author", "images", "created_at", "updated_at")
	imagesValue, ok := noteValue["images"].([]any)
	if !ok || len(imagesValue) != 0 {
		t.Fatalf("wire note images = %#v, want empty array", noteValue["images"])
	}
	authorValue, ok := noteValue["author"].(map[string]any)
	if !ok {
		t.Fatalf("wire author = %T, want object", noteValue["author"])
	}
	requireExactJSONKeys(t, authorValue, "id", "display_name")
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestListAuthorNotesReturnsCursorForLongLegacyID(t *testing.T) {
	createdAt := time.UnixMilli(exampleAuthorCreatedMS).UTC()
	longID := strings.Repeat("😀", 100)
	router := NewRouter(fakeNoteStore{
		listAuthorNotes: func(context.Context, note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			return note.AuthorNotesPage{
				Notes:   []note.AuthorNote{authorHTTPAuthorNote(longID, createdAt, exampleCursorPageKey)},
				HasMore: true,
			}, nil
		},
	}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay, NoteCount: 2}, nil
		},
	}, DefaultAuthLimits())

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID)+"/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.AuthorNotesPage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.NextCursor == nil {
		t.Fatal("next_cursor = nil, want cursor")
	}
	cursor, problems := decodeAuthorNotesCursor(body.NextCursor)
	if len(problems) > 0 {
		t.Fatalf("returned cursor did not decode: %#v", problems)
	}
	if cursor == nil || cursor.PageKey != exampleCursorPageKey {
		t.Fatalf("decoded cursor = %#v, want page key %d", cursor, exampleCursorPageKey)
	}
}

func TestListAuthorNotesPassesExplicitLimitAndCursor(t *testing.T) {
	createdAt := time.UnixMilli(exampleAuthorCreatedMS).UTC()
	encoded, err := encodeAuthorNotesCursor(note.AuthorNotePosition{CreatedAt: createdAt, PageKey: exampleCursorPageKey})
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	router := NewRouter(fakeNoteStore{
		listAuthorNotes: func(_ context.Context, input note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			if input.Limit != 2 {
				t.Fatalf("limit = %d, want 2", input.Limit)
			}
			if input.After == nil {
				t.Fatal("cursor = nil, want decoded cursor")
			}
			if input.After.PageKey != exampleCursorPageKey || !input.After.CreatedAt.Equal(createdAt) {
				t.Fatalf("cursor = %#v, want request cursor", input.After)
			}
			return note.AuthorNotesPage{Notes: []note.AuthorNote{}}, nil
		},
	}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay, NoteCount: 0}, nil
		},
	}, DefaultAuthLimits())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID)+"/notes?limit=2&cursor="+encoded, nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.AuthorNotesPage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Notes) != 0 {
		t.Fatalf("note count = %d, want 0", len(body.Notes))
	}
	if body.NextCursor != nil {
		t.Fatalf("next_cursor = %q, want nil", *body.NextCursor)
	}
}
func TestAuthorNotesCursorRoundTripsPageKey(t *testing.T) {
	tests := []struct {
		name     string
		position note.AuthorNotePosition
	}{
		{
			name:     "positive created at",
			position: note.AuthorNotePosition{CreatedAt: time.UnixMilli(exampleAuthorCreatedMS).UTC(), PageKey: exampleCursorPageKey},
		},
		{
			name:     "zero created at",
			position: note.AuthorNotePosition{CreatedAt: time.UnixMilli(0).UTC(), PageKey: exampleCursorPageKey},
		},
		{
			name:     "negative created at",
			position: note.AuthorNotePosition{CreatedAt: time.UnixMilli(-1).UTC(), PageKey: exampleCursorPageKey},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeAuthorNotesCursor(tt.position)
			if err != nil {
				t.Fatalf("encode cursor: %v", err)
			}
			decoded, problems := decodeAuthorNotesCursor(&encoded)
			if diff := cmp.Diff([]note.ValidationProblem(nil), problems); diff != "" {
				t.Fatalf("problems mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(&tt.position, decoded); diff != "" {
				t.Fatalf("cursor mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListAuthorNotesRejectsInvalidParametersBeforeAuthorLookup(t *testing.T) {
	unsupportedVersion := rawAuthorCursor(`{"v":2,"created_at":1782993600000,"page_key":123}`)
	tests := []struct {
		name      string
		query     string
		wantField openapi.ValidationField
	}{
		{name: "zero limit", query: "limit=0", wantField: openapi.ValidationFieldLimit},
		{name: "over max limit", query: "limit=51", wantField: openapi.ValidationFieldLimit},
		{name: "non-integer limit", query: "limit=abc", wantField: openapi.ValidationFieldLimit},
		{name: "malformed cursor", query: "cursor=not-base64!", wantField: openapi.ValidationFieldCursor},
		{name: "unsupported cursor version", query: "cursor=" + unsupportedVersion, wantField: openapi.ValidationFieldCursor},
		{name: "oversized cursor", query: "cursor=" + strings.Repeat("a", maxAuthorNotesCursorLength+1), wantField: openapi.ValidationFieldCursor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(fakeNoteStore{
				listAuthorNotes: func(context.Context, note.AuthorNotesInput) (note.AuthorNotesPage, error) {
					t.Fatal("ListAuthorNotes should not be called")
					return note.AuthorNotesPage{}, nil
				},
			}, fakeCatalog{}, fakeUserStore{
				findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
					t.Fatal("FindPublicAuthor should not be called")
					return author.PublicAuthor{}, nil
				},
			}, DefaultAuthLimits())
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID)+"/notes?"+tt.query, nil)

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidNote {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidNote)
			}
			requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{{Field: tt.wantField, Code: openapi.ValidationProblemCodeInvalid}})
		})
	}
}

func TestListAuthorNotesReturnsNotFoundForUnknownAuthor(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		listAuthorNotes: func(context.Context, note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			t.Fatal("ListAuthorNotes should not be called")
			return note.AuthorNotesPage{}, nil
		},
	}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{}, author.ErrAuthorNotFound
		},
	}, DefaultAuthLimits())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/missing-author/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
}

func TestListAuthorNotesReturnsInternalError(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		listAuthorNotes: func(context.Context, note.AuthorNotesInput) (note.AuthorNotesPage, error) {
			return note.AuthorNotesPage{}, errors.New("database unavailable")
		},
	}, fakeCatalog{}, fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay, NoteCount: 1}, nil
		},
	}, DefaultAuthLimits())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/authors/"+string(exampleAuthorID)+"/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	requireErrorCode(t, response, openapi.ErrorCodeInternal)
}

func TestAuthorNotesCursorRejectsMalformedPayloads(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{name: "empty", cursor: ""},
		{name: "invalid base64", cursor: "not-base64!"},
		{name: "unsupported version", cursor: rawAuthorCursor(`{"v":2,"created_at":1782993600000,"page_key":123}`)},
		{name: "missing version", cursor: rawAuthorCursor(`{"created_at":1782993600000,"page_key":123}`)},
		{name: "missing created at", cursor: rawAuthorCursor(`{"v":1,"page_key":123}`)},
		{name: "missing page key", cursor: rawAuthorCursor(`{"v":1,"created_at":1782993600000}`)},
		{name: "non-positive page key", cursor: rawAuthorCursor(`{"v":1,"created_at":1782993600000,"page_key":0}`)},
		{name: "unknown field", cursor: rawAuthorCursor(`{"v":1,"created_at":1782993600000,"page_key":123,"extra":true}`)},
		{name: "trailing json", cursor: rawAuthorCursor(`{"v":1,"created_at":1782993600000,"page_key":123}{}`)},
		{name: "oversized", cursor: strings.Repeat("a", maxAuthorNotesCursorLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor, problems := decodeAuthorNotesCursor(&tt.cursor)
			if cursor != nil {
				t.Fatalf("cursor = %#v, want nil", cursor)
			}
			want := []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
			if diff := cmp.Diff(want, problems); diff != "" {
				t.Fatalf("problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func authorHTTPNote(id string, createdAt time.Time) note.Note {
	return note.Note{
		ID:           id,
		UserID:       "user-id-marina",
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: note.CategorySlugFood,
		PlaceSlug:    note.PlaceSlugSaoPaulo,
		Author:       note.AuthorSummary{ID: exampleAuthorID, DisplayName: exampleAuthorDisplay},
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
}

func authorHTTPAuthorNote(id string, createdAt time.Time, pageKey int64) note.AuthorNote {
	found := authorHTTPNote(id, createdAt)
	return note.AuthorNote{
		Note: found,
		Position: note.AuthorNotePosition{
			CreatedAt: createdAt,
			PageKey:   pageKey,
		},
	}
}

func rawAuthorCursor(raw string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func requireExactJSONKeys(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()
	want := make(map[string]bool, len(keys))
	for _, key := range keys {
		want[key] = true
	}
	for key := range want {
		if _, ok := value[key]; !ok {
			t.Fatalf("missing JSON key %q in %#v", key, value)
		}
	}
	for key := range value {
		if !want[key] {
			t.Fatalf("unexpected JSON key %q in %#v", key, value)
		}
	}
}

func requireNoPrivateWireFields(t *testing.T, body string) {
	t.Helper()
	privateFragments := []string{"user_id", "username", "account_state", "login_identity", "secret_hash", "password", "token", "session_id", "session_expiry", "expires_at"}
	for _, fragment := range privateFragments {
		if strings.Contains(body, fragment) {
			t.Fatalf("response contains private fragment %q: %s", fragment, body)
		}
	}
}

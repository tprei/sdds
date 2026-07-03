package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func TestListNotesReturnsRecentNotes(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := NewRouter(fakeNoteStore{
		listNotes: func(_ context.Context, limit int) ([]note.Note, error) {
			if limit != recentNotesLimit {
				t.Fatalf("limit = %d, want %d", limit, recentNotesLimit)
			}
			return []note.Note{{
				ID:           "018ff5b8-0000-7000-8000-000000000000",
				Title:        "Café bom",
				Body:         "Tem pão de queijo decente.",
				CategorySlug: "comida",
				CitySlug:     "sao-paulo",
				CreatedAt:    now,
				UpdatedAt:    now,
			}}, nil
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body openapi.ListNotesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Notes) != 1 {
		t.Fatalf("note count = %d, want 1", len(body.Notes))
	}
	if body.Notes[0].CategorySlug != string(note.CategorySlugComida) {
		t.Fatalf("category_slug = %s, want %s", body.Notes[0].CategorySlug, note.CategorySlugComida)
	}
	if body.Notes[0].CreatedAt != now.UnixMilli() {
		t.Fatalf("created_at = %d, want %d", body.Notes[0].CreatedAt, now.UnixMilli())
	}
	if body.Notes[0].UpdatedAt != now.UnixMilli() {
		t.Fatalf("updated_at = %d, want %d", body.Notes[0].UpdatedAt, now.UnixMilli())
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	notesValue, ok := wireBody["notes"].([]any)
	if !ok {
		t.Fatalf("notes = %T, want []any", wireBody["notes"])
	}
	if len(notesValue) != 1 {
		t.Fatalf("wire note count = %d, want 1", len(notesValue))
	}

	noteValue, ok := notesValue[0].(map[string]any)
	if !ok {
		t.Fatalf("note = %T, want map[string]any", notesValue[0])
	}
	requireJSONKeys(t, noteValue, "id", "title", "body", "category_slug", "city_slug", "created_at", "updated_at")
	requireJSONNumber(t, noteValue, "created_at", now.UnixMilli())
	requireJSONNumber(t, noteValue, "updated_at", now.UnixMilli())
}

func TestCreateNoteReturnsCreatedNote(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	router := NewRouter(fakeNoteStore{
		createNote: func(_ context.Context, input note.CreateInput) (note.Note, error) {
			if input.Title != "Café bom" {
				t.Fatalf("title = %q, want Café bom", input.Title)
			}
			if input.CategorySlug != "comida" {
				t.Fatalf("category = %q, want comida", input.CategorySlug)
			}
			return note.Note{
				ID:           "018ff5b8-0000-7000-8000-000000000000",
				Title:        input.Title,
				Body:         input.Body,
				CategorySlug: input.CategorySlug,
				CitySlug:     input.CitySlug,
				CreatedAt:    now,
				UpdatedAt:    now,
			}, nil
		},
	})

	requestBody := []byte(`{"title":" Café bom ","body":"Tem pão de queijo decente.","category_slug":"comida","city_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var body openapi.Note
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Id == "" {
		t.Fatal("id is empty")
	}
	if body.CitySlug != string(note.CitySlugSaoPaulo) {
		t.Fatalf("city_slug = %s, want %s", body.CitySlug, note.CitySlugSaoPaulo)
	}
	if body.CreatedAt != now.UnixMilli() {
		t.Fatalf("created_at = %d, want %d", body.CreatedAt, now.UnixMilli())
	}
	if body.UpdatedAt != now.UnixMilli() {
		t.Fatalf("updated_at = %d, want %d", body.UpdatedAt, now.UnixMilli())
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireJSONNumber(t, wireBody, "created_at", now.UnixMilli())
	requireJSONNumber(t, wireBody, "updated_at", now.UnixMilli())
}

func TestCreateNoteRejectsValidationProblems(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"   ","body":"   ","category_slug":"comida","city_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

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
	if body.Fields == nil {
		t.Fatal("fields is nil")
	}
	if len(*body.Fields) != 2 {
		t.Fatalf("field count = %d, want 2", len(*body.Fields))
	}
	if (*body.Fields)[0].Code == "" {
		t.Fatal("first field code is empty")
	}

	wireBody := decodeResponseObject(t, response.Body.Bytes())
	requireJSONKeys(t, wireBody, "code", "fields")
	if _, ok := wireBody["error"]; ok {
		t.Fatal("unexpected error key in response body")
	}

	fieldsValue, ok := wireBody["fields"].([]any)
	if !ok {
		t.Fatalf("fields = %T, want []any", wireBody["fields"])
	}
	if len(fieldsValue) != 2 {
		t.Fatalf("wire field count = %d, want 2", len(fieldsValue))
	}

	firstField, ok := fieldsValue[0].(map[string]any)
	if !ok {
		t.Fatalf("first field = %T, want map[string]any", fieldsValue[0])
	}
	requireJSONKeys(t, firstField, "field", "code")
}

func TestCreateNoteRejectsOpenAPIRequestSchemaProblems(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "title too long",
			body: `{"title":"` + strings.Repeat("a", note.TitleMaxLength+1) + `","body":"Funciona.","category_slug":"comida","city_slug":"sao-paulo"}`,
		},
		{
			name: "body too long",
			body: `{"title":"Café bom","body":"` + strings.Repeat("a", note.BodyMaxLength+1) + `","category_slug":"comida","city_slug":"sao-paulo"}`,
		},
	}

	router := NewRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidJSON {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
			}
			if body.Fields != nil {
				t.Fatalf("fields = %#v, want nil", *body.Fields)
			}
		})
	}
}

func TestCreateNoteRejectsUnknownSlugsThroughDomainValidation(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"qualquer","city_slug":"qualquer"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

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
	if body.Fields == nil {
		t.Fatal("fields is nil")
	}
	if len(*body.Fields) != 2 {
		t.Fatalf("field count = %d, want 2", len(*body.Fields))
	}
	requireValidationProblem(t, *body.Fields, openapi.ValidationFieldCategorySlug, openapi.ValidationProblemCodeUnknown)
	requireValidationProblem(t, *body.Fields, openapi.ValidationFieldCitySlug, openapi.ValidationProblemCodeUnknown)
}

func TestCreateNoteRejectsInvalidJSON(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader([]byte(`{"title":`)))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsMissingOrUnsupportedContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "missing"},
		{name: "unsupported", contentType: "text/plain"},
	}

	router := NewRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := `{"title":"Café bom","body":"Funciona.","category_slug":"comida","city_slug":"sao-paulo"}`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/notes", strings.NewReader(requestBody))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidJSON {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
			}
		})
	}
}

func TestCreateNoteRejectsOldMobileShapedJSON(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category":"comida","city":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsUnknownJSONFields(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		createNote: func(context.Context, note.CreateInput) (note.Note, error) {
			t.Fatal("CreateNote should not be called")
			return note.Note{}, nil
		},
	})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"comida","city_slug":"sao-paulo","unexpected":true}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsTrailingJSON(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"Funciona.","category_slug":"comida","city_slug":"sao-paulo"} {}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestCreateNoteRejectsOversizedRequestBody(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	requestBody := []byte(`{"title":"Café bom","body":"` + strings.Repeat("a", int(maxCreateNoteRequestBytes)) + `","category_slug":"comida","city_slug":"sao-paulo"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/notes", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeRequestTooLarge)
	}
}

func TestListNotesRejectsRequestBody(t *testing.T) {
	router := NewRouter(fakeNoteStore{
		listNotes: func(context.Context, int) ([]note.Note, error) {
			t.Fatal("ListRecentNotes should not be called")
			return nil, nil
		},
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", strings.NewReader(`{"unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidJSON)
	}
}

func TestNoteRoutesRejectUnsupportedMethods(t *testing.T) {
	router := NewRouter(fakeNoteStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/notes", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func requireOpenAPIResponse(t *testing.T, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()

	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("load OpenAPI spec: %v", err)
	}
	spec.Servers = nil

	router, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatalf("build OpenAPI router: %v", err)
	}

	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route: %v", err)
	}

	options := &openapi3filter.Options{
		AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		IncludeResponseStatus: true,
	}
	err = openapi3filter.ValidateResponse(request.Context(), (&openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: pathParams,
			Route:      route,
			Options:    options,
		},
		Status:  response.Code,
		Header:  response.Header(),
		Options: options,
	}).SetBodyBytes(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("response does not match OpenAPI contract: %v", err)
	}
}

func decodeResponseObject(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode wire response: %v", err)
	}
	return value
}

func requireJSONNumber(t *testing.T, value map[string]any, key string, want int64) {
	t.Helper()

	got, ok := value[key].(float64)
	if !ok {
		t.Fatalf("%s = %T, want JSON number", key, value[key])
	}
	if got != float64(want) {
		t.Fatalf("%s = %v, want %d", key, got, want)
	}
}

func requireValidationProblem(t *testing.T, problems []openapi.ValidationProblem, field openapi.ValidationField, code openapi.ValidationProblemCode) {
	t.Helper()

	for _, problem := range problems {
		if problem.Field == field && problem.Code == code {
			return
		}
	}
	t.Fatalf("missing validation problem %s/%s in %#v", field, code, problems)
}

func requireJSONKeys(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := value[key]; !ok {
			t.Fatalf("missing JSON key %q in %#v", key, value)
		}
	}
}

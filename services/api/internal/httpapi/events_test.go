package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const eventTestUserID user.UserID = "00000000-0000-0000-0000-000000000001"

func eventOccurredAt() time.Time {
	return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
}

func newEventHTTPRouter(t *testing.T, store EventStore, limits EventLimits, clock func() time.Time, authenticated bool) http.Handler {
	return newEventHTTPRouterWithSessionError(t, store, limits, clock, authenticated, nil)
}

func newEventHTTPRouterWithSessionError(t *testing.T, store EventStore, limits EventLimits, clock func() time.Time, authenticated bool, sessionError error) http.Handler {
	t.Helper()
	users := fakeUserStore{findCurrentSession: func(_ context.Context, tokenHash string, _ time.Time) (user.CurrentSession, error) {
		if sessionError != nil {
			return user.CurrentSession{}, sessionError
		}
		if tokenHash != user.HashSessionToken("current-token") {
			return user.CurrentSession{}, user.ErrSessionNotFound
		}
		return user.CurrentSession{
			Session: user.Session{UserID: eventTestUserID, TokenHash: tokenHash},
			User:    user.User{ID: eventTestUserID, State: user.UserStateActive},
		}, nil
	}}
	hasher := user.NewPasswordHasher()
	handler := newRouter(
		noteHandlers{noteStore: fakeNoteStore{}, notePublisher: fakeNoteStore{}, noteSearcher: fakeNoteStore{}, authorNoteStore: fakeNoteStore{}, usefulStore: fakeNoteStore{}, categoryCatalog: fakeCatalog{}},
		commentHandlers{store: fakeCommentStore{}, notes: fakeNoteStore{}},
		reportHandlers{store: fakeReportStore{}, notes: fakeNoteStore{}, comments: fakeCommentStore{}},
		eventHandlers{store: store, limits: newEventRateLimiters(limits, clock), clock: clock},
		authHandlers{
			users:                 users,
			publicAuthors:         users,
			contactChannels:       fakeContactChannelStore{},
			passwordHasher:        hasher,
			invalidCredentialHash: mustInvalidCredentialHash(hasher),
			rateLimiters:          newAuthRateLimiters(DefaultAuthLimits(), clock),
			newSessionToken:       user.NewSessionToken,
			clock:                 clock,
		},
		mediaHandlers{imageUploads: fakeUploadPreparer{}, attachedImages: fakeAttachedImageReader{}},
		systemHandlers{readiness: fakeReadiness{}},
		newPublicReadRateLimiters(DefaultPublicReadLimits(), clock),
	)
	if authenticated {
		return withCurrentSessionHeader(handler)
	}
	return handler
}

func eventTestID(number int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", number)
}

func validNotePublishedEvent(id string) map[string]any {
	return map[string]any{
		"id":              id,
		"kind":            string(event.KindNotePublished),
		"occurred_at":     eventOccurredAt().UnixMilli(),
		"installation_id": eventTestID(900),
		"platform":        "web",
		"app_version":     "0.0.1",
		"schema_version":  1,
		"payload": map[string]any{
			"note_id":       eventTestID(100),
			"category_slug": "beauty",
		},
	}
}

func validSearchSubmittedEvent(id string) map[string]any {
	return map[string]any{
		"id":              id,
		"kind":            string(event.KindSearchSubmitted),
		"occurred_at":     eventOccurredAt().UnixMilli(),
		"installation_id": eventTestID(900),
		"platform":        "web",
		"app_version":     "0.0.1",
		"schema_version":  1,
		"payload": map[string]any{
			"search_id":      eventTestID(200),
			"search_version": "fts5-v1",
			"query":          "café",
			"category_slug":  nil,
		},
	}
}
func validSearchNoResultsEvent(id string) map[string]any {
	return map[string]any{
		"id":              id,
		"kind":            string(event.KindSearchNoResults),
		"occurred_at":     eventOccurredAt().UnixMilli(),
		"installation_id": eventTestID(900),
		"platform":        "web",
		"app_version":     "0.0.1",
		"schema_version":  1,
		"payload": map[string]any{
			"search_id":      eventTestID(200),
			"search_version": "fts5-v1",
			"query":          "café",
			"category_slug":  nil,
			"result_count":   0,
		},
	}
}

func eventRequestBody(t *testing.T, events ...map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		t.Fatalf("marshal event request: %v", err)
	}
	return string(body)
}

func eventRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return jsonRequest(http.MethodPost, "/v1/events", body)
}

func eventErrorResponse(t *testing.T, response *httptest.ResponseRecorder) openapi.EventErrorResponse {
	t.Helper()
	var result openapi.EventErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode event error: %v; body=%s", err, response.Body.String())
	}
	return result
}

func TestCreateEventsAcceptsValidEnvelopeAndDerivesUser(t *testing.T) {
	receivedAt := time.Date(2026, 7, 26, 12, 34, 56, 789000000, time.FixedZone("local", -3*60*60))
	var got []event.Record
	var gotReceivedAt time.Time
	store := fakeEventStore{appendBatch: func(_ context.Context, records []event.Record, received time.Time) (event.AppendBatchResult, error) {
		got = append([]event.Record(nil), records...)
		gotReceivedAt = received
		return event.AppendBatchResult{AcceptedCount: len(records)}, nil
	}}
	router := newEventHTTPRouter(t, store, EventLimits{UserEventsPerMinute: 600, GlobalEventsPerMinute: 6000}, func() time.Time { return receivedAt }, true)
	request := eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(1))))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var receipt openapi.CreateEventsReceipt
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.AcceptedCount != 1 || receipt.DuplicateCount != 0 {
		t.Fatalf("receipt = %+v, want one accepted event", receipt)
	}
	if len(got) != 1 {
		t.Fatalf("stored records = %d, want 1", len(got))
	}
	if got[0].UserID != eventTestUserID {
		t.Fatalf("stored user ID = %q, want server identity %q", got[0].UserID, eventTestUserID)
	}
	if got[0].OccurredAt.UnixMilli() != eventOccurredAt().UnixMilli() {
		t.Fatalf("occurred_at = %d, want client timestamp", got[0].OccurredAt.UnixMilli())
	}
	wantReceivedAt := receivedAt.UTC().Truncate(time.Millisecond)
	if !gotReceivedAt.Equal(wantReceivedAt) {
		t.Fatalf("received_at = %v, want %v", gotReceivedAt, wantReceivedAt)
	}
}
func TestCreateEventsRejectsUnknownNestedContextKeysWithContractPath(t *testing.T) {
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		t.Fatal("event store must not be called")
		return event.AppendBatchResult{}, nil
	}}
	value := validNotePublishedEvent(eventTestID(13))
	value["kind"] = string(event.KindNoteMarkedUseful)
	value["payload"] = map[string]any{
		"note_id": eventTestID(100),
		"context": map[string]any{
			"source":     "note_detail",
			"unexpected": true,
		},
	}
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), time.Now, true)
	request := eventRequest(t, eventRequestBody(t, value))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	result := eventErrorResponse(t, response)
	if len(result.Problems) != 1 || result.Problems[0].Field != "payload.context.unexpected" {
		t.Fatalf("problems = %+v, want payload.context.unexpected", result.Problems)
	}
}

func TestCreateEventsRejectsSpoofedUserAndInsertsNothing(t *testing.T) {
	called := false
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		called = true
		return event.AppendBatchResult{}, nil
	}}
	eventWithSpoofedUser := validNotePublishedEvent(eventTestID(2))
	eventWithSpoofedUser["user_id"] = eventTestID(999)
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), time.Now, true)
	request := eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(1)), eventWithSpoofedUser))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	result := eventErrorResponse(t, response)
	if result.Code != openapi.InvalidEvent || len(result.Problems) != 1 {
		t.Fatalf("event error = %+v, want one invalid user_id problem", result)
	}
	problem := result.Problems[0]
	if problem.Index != 1 || problem.Field != "user_id" || problem.Code != openapi.Unknown {
		t.Fatalf("problem = %+v, want index 1 unknown user_id", problem)
	}
	if called {
		t.Fatal("event store called for an atomic invalid batch")
	}
}

func TestCreateEventsRejectsInvalidJSONAndBatchBounds(t *testing.T) {
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		return event.AppendBatchResult{}, errors.New("store must not be called")
	}}
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), time.Now, true)
	tests := []struct {
		name string
		body string
		code openapi.ErrorCode
	}{
		{name: "malformed JSON", body: `{"events":[`, code: openapi.ErrorCodeInvalidJSON},
		{name: "empty array", body: `{"events":[]}`, code: openapi.ErrorCodeInvalidEventBatch},
		{name: "null array", body: `{"events":null}`, code: openapi.ErrorCodeInvalidJSON},
		{name: "missing events", body: `{}`, code: openapi.ErrorCodeInvalidJSON},
		{name: "case-sensitive root key", body: `{"Events":[]}`, code: openapi.ErrorCodeInvalidJSON},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := eventRequest(t, test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			var result openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if result.Code != test.code {
				t.Fatalf("code = %q, want %q", result.Code, test.code)
			}
		})
	}
	request := eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(100))))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong content type status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var contentTypeError openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &contentTypeError); err != nil {
		t.Fatalf("decode content-type error: %v", err)
	}
	if contentTypeError.Code != openapi.ErrorCodeInvalidJSON {
		t.Fatalf("wrong content type code = %q, want %q", contentTypeError.Code, openapi.ErrorCodeInvalidJSON)
	}

	tooMany := make([]map[string]any, 51)
	for index := range tooMany {
		tooMany[index] = validNotePublishedEvent(eventTestID(index + 1))
	}
	request = eventRequest(t, eventRequestBody(t, tooMany...))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("too-many status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var tooManyError openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &tooManyError); err != nil {
		t.Fatalf("decode too-many error: %v", err)
	}
	if tooManyError.Code != openapi.ErrorCodeInvalidEventBatch {
		t.Fatalf("too-many code = %q, want %q", tooManyError.Code, openapi.ErrorCodeInvalidEventBatch)
	}
}

func TestCreateEventsRejectsUnknownKindFutureSchemaAndOutOfRangeTimestamp(t *testing.T) {
	storeCalls := 0
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		storeCalls++
		return event.AppendBatchResult{}, nil
	}}
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), time.Now, true)
	tests := []struct {
		name  string
		event map[string]any
		field string
		code  openapi.InvalidEventProblemCode
	}{
		{
			name: "unknown kind",
			event: func() map[string]any {
				value := validNotePublishedEvent(eventTestID(1))
				value["kind"] = "client_defined_kind"
				return value
			}(),
			field: "kind",
			code:  openapi.Unknown,
		},
		{
			name: "future schema",
			event: func() map[string]any {
				value := validNotePublishedEvent(eventTestID(2))
				value["schema_version"] = 2
				return value
			}(),
			field: "schema_version",
			code:  openapi.Unsupported,
		},
		{
			name: "timestamp below range",
			event: func() map[string]any {
				value := validNotePublishedEvent(eventTestID(3))
				value["occurred_at"] = 0
				return value
			}(),
			field: "occurred_at",
			code:  openapi.Invalid,
		},
		{
			name: "missing payload field",
			event: func() map[string]any {
				value := validSearchSubmittedEvent(eventTestID(4))
				delete(value["payload"].(map[string]any), "query")
				return value
			}(),
			field: "payload.query",
			code:  openapi.Required,
		},
		{
			name: "null required result count",
			event: func() map[string]any {
				value := validSearchNoResultsEvent(eventTestID(5))
				value["payload"].(map[string]any)["result_count"] = nil
				return value
			}(),
			field: "payload.result_count",
			code:  openapi.Invalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := eventRequest(t, eventRequestBody(t, test.event))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			result := eventErrorResponse(t, response)
			found := false
			for _, problem := range result.Problems {
				if problem.Index == 0 && problem.Field == test.field && problem.Code == test.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("problems = %+v, want %s/%s", result.Problems, test.field, test.code)
			}
		})
	}
	if storeCalls != 0 {
		t.Fatalf("store calls = %d, want 0", storeCalls)
	}
}

func TestCreateEventsRejectsOversizedPayloadAndHTTPBody(t *testing.T) {
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		return event.AppendBatchResult{}, errors.New("store must not be called")
	}}
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), time.Now, true)
	largeQuery := strings.Repeat("a", eventMaxPayloadBytes)
	largeEvent := validSearchSubmittedEvent(eventTestID(1))
	largeEvent["payload"].(map[string]any)["query"] = largeQuery
	request := eventRequest(t, eventRequestBody(t, largeEvent))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("large payload status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	result := eventErrorResponse(t, response)
	foundTooLarge := false
	for _, problem := range result.Problems {
		if problem.Field == "payload" && problem.Code == openapi.TooLarge {
			foundTooLarge = true
		}
	}
	if !foundTooLarge {
		t.Fatalf("large payload problems = %+v, want payload too_large", result.Problems)
	}
	whitespacePayload := fmt.Sprintf(
		`{"events":[{"id":%q,"kind":"search_submitted","occurred_at":%d,"installation_id":%q,"platform":"web","app_version":"0.0.1","schema_version":1,"payload":%s{"search_id":%q,"search_version":"fts5-v1","query":"ok","category_slug":null}}]}`,
		eventTestID(2),
		eventOccurredAt().UnixMilli(),
		eventTestID(900),
		strings.Repeat(" ", eventMaxPayloadBytes),
		eventTestID(200),
	)
	request = eventRequest(t, whitespacePayload)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("whitespace payload status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	whitespaceResult := eventErrorResponse(t, response)
	foundWhitespaceTooLarge := false
	for _, problem := range whitespaceResult.Problems {
		if problem.Field == "payload" && problem.Code == openapi.TooLarge {
			foundWhitespaceTooLarge = true
		}
	}
	if !foundWhitespaceTooLarge {
		t.Fatalf("whitespace payload problems = %+v, want payload too_large", whitespaceResult.Problems)
	}

	trailingBody := string(eventRequestBody(t, validNotePublishedEvent(eventTestID(3)))) + strings.Repeat(" ", eventMaxBodyBytes)
	request = eventRequest(t, trailingBody)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("trailing body status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	var trailingBodyError openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &trailingBodyError); err != nil {
		t.Fatalf("decode trailing body error: %v", err)
	}
	if trailingBodyError.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("trailing body code = %q, want %q", trailingBodyError.Code, openapi.ErrorCodeRequestTooLarge)
	}

	oversizedBody := `{"events":[` + strings.Repeat(" ", eventMaxBodyBytes) + `]}`
	request = eventRequest(t, oversizedBody)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	var bodyError openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &bodyError); err != nil {
		t.Fatalf("decode large body error: %v", err)
	}
	if bodyError.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("large body code = %q, want %q", bodyError.Code, openapi.ErrorCodeRequestTooLarge)
	}
	request = eventRequest(t, oversizedBody)
	request.Header.Set("Content-Type", "text/plain")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large wrong-content-type body status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestCreateEventsRequiresAuthentication(t *testing.T) {
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		return event.AppendBatchResult{}, errors.New("store must not be called")
	}}
	clock := func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) }
	router := newEventHTTPRouter(t, store, DefaultEventLimits(), clock, false)
	request := eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(1))))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	var result openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode auth error: %v", err)
	}
	if result.Code != openapi.ErrorCodeUnauthenticated {
		t.Fatalf("code = %q, want %q", result.Code, openapi.ErrorCodeUnauthenticated)
	}
}

func TestCreateEventsRejectsExpiredSession(t *testing.T) {
	store := fakeEventStore{appendBatch: func(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error) {
		return event.AppendBatchResult{}, errors.New("store must not be called")
	}}
	clock := func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) }
	router := newEventHTTPRouterWithSessionError(t, store, DefaultEventLimits(), clock, true, user.ErrSessionExpired)
	request := eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(1))))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	var result openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode auth error: %v", err)
	}
	if result.Code != openapi.ErrorCodeUnauthenticated {
		t.Fatalf("code = %q, want %q", result.Code, openapi.ErrorCodeUnauthenticated)
	}
}

func TestEventRateLimitReservesBatchCountAndReportsRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	perUser := newEventRateLimiters(EventLimits{UserEventsPerMinute: 50, GlobalEventsPerMinute: 100}, func() time.Time { return now })
	if retryAfter, ok := perUser.reserve(now, string(eventTestUserID), 50); !ok || retryAfter != 0 {
		t.Fatalf("50-event reservation = (%d, %t), want (0, true)", retryAfter, ok)
	}
	if retryAfter, ok := perUser.reserve(now, string(eventTestUserID), 1); ok || retryAfter != 2 {
		t.Fatalf("per-user retry = (%d, %t), want (2, false)", retryAfter, ok)
	}

	global := newEventRateLimiters(EventLimits{UserEventsPerMinute: 100, GlobalEventsPerMinute: 1}, func() time.Time { return now })
	if retryAfter, ok := global.reserve(now, string(eventTestUserID), 1); !ok || retryAfter != 0 {
		t.Fatalf("global first reservation = (%d, %t), want (0, true)", retryAfter, ok)
	}
	if retryAfter, ok := global.reserve(now, eventTestID(2), 1); ok || retryAfter != 60 {
		t.Fatalf("global retry = (%d, %t), want (60, false)", retryAfter, ok)
	}
}

func TestCreateEventsRateLimitChargesBatchAndSetsHeader(t *testing.T) {
	store := fakeEventStore{appendBatch: func(_ context.Context, records []event.Record, _ time.Time) (event.AppendBatchResult, error) {
		return event.AppendBatchResult{AcceptedCount: len(records)}, nil
	}}
	clock := func() time.Time { return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC) }
	router := newEventHTTPRouter(t, store, EventLimits{UserEventsPerMinute: 50, GlobalEventsPerMinute: 100}, clock, true)
	batch := make([]map[string]any, 50)
	for index := range batch {
		batch[index] = validNotePublishedEvent(eventTestID(index + 1))
	}
	request := eventRequest(t, eventRequestBody(t, batch...))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("batch status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	request = eventRequest(t, eventRequestBody(t, validNotePublishedEvent(eventTestID(1000))))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response.Header().Get("Retry-After") != "2" {
		t.Fatalf("Retry-After = %q, want 2", response.Header().Get("Retry-After"))
	}
}

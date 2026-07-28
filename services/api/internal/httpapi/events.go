package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	eventMaxBodyBytes    = 256 * 1024
	eventMaxPayloadBytes = 8 * 1024
)

type EventStore interface {
	AppendBatch(context.Context, []event.Record, time.Time) (event.AppendBatchResult, error)
}

type EventLimits struct {
	UserEventsPerMinute   int
	GlobalEventsPerMinute int
}

func DefaultEventLimits() EventLimits {
	return EventLimits{UserEventsPerMinute: 600, GlobalEventsPerMinute: 6000}
}

type EventDependencies struct {
	Store  EventStore
	Limits EventLimits
}

type eventHandlers struct {
	store  EventStore
	limits eventRateLimiters
	clock  func() time.Time
}

type eventDecodeResult struct {
	records         []event.Record
	problems        []openapi.InvalidEventProblem
	invalidJSON     bool
	invalidBatch    bool
	requestTooLarge bool
}

func (handler server) CreateEvents(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	decoded := decodeEventsRequest(w, r, current.User.ID)
	switch {
	case decoded.requestTooLarge:
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
		return
	case decoded.invalidJSON:
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
		return
	case decoded.invalidBatch:
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidEventBatch})
		return
	case len(decoded.problems) > 0:
		writeJSON(w, http.StatusBadRequest, openapi.EventErrorResponse{
			Code:     openapi.InvalidEvent,
			Problems: decoded.problems,
		})
		return
	}

	now := handler.events.clock()
	retryAfter, allowed := handler.events.limits.reserve(now, string(current.User.ID), len(decoded.records))
	if !allowed {
		writeEventRateLimited(w, retryAfter)
		return
	}
	result, err := handler.events.store.AppendBatch(r.Context(), decoded.records, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusOK, openapi.CreateEventsReceipt{
		AcceptedCount:  result.AcceptedCount,
		DuplicateCount: result.DuplicateCount,
	})
}

func validEventContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func decodeEventsRequest(w http.ResponseWriter, r *http.Request, userID user.UserID) eventDecodeResult {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, eventMaxBodyBytes))
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return eventDecodeResult{requestTooLarge: true}
		}
		return eventDecodeResult{invalidJSON: true}
	}
	if !utf8.Valid(body) {
		return eventDecodeResult{invalidJSON: true}
	}
	if !validEventContentType(r.Header.Get("Content-Type")) {
		return eventDecodeResult{invalidJSON: true}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return eventDecodeResult{invalidJSON: true}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return eventDecodeResult{invalidJSON: true}
	}
	if fields == nil || len(fields) != 1 {
		return eventDecodeResult{invalidJSON: true}
	}
	requestEvents, ok := fields["events"]
	if !ok || len(requestEvents) == 0 || bytes.Equal(bytes.TrimSpace(requestEvents), []byte("null")) {
		return eventDecodeResult{invalidJSON: true}
	}

	var rawEvents []json.RawMessage
	if err := json.Unmarshal(requestEvents, &rawEvents); err != nil || rawEvents == nil {
		return eventDecodeResult{invalidJSON: true}
	}
	if len(rawEvents) < 1 || len(rawEvents) > 50 {
		return eventDecodeResult{invalidBatch: true}
	}

	result := eventDecodeResult{records: make([]event.Record, 0, len(rawEvents))}
	for index, rawEvent := range rawEvents {
		record, problems := decodeEventItem(rawEvent, index, userID)
		result.problems = append(result.problems, problems...)
		if len(problems) == 0 {
			result.records = append(result.records, record)
		}
	}
	if len(result.problems) > 0 {
		result.records = nil
	}
	return result
}

func decodeEventItem(rawEvent json.RawMessage, index int, userID user.UserID) (event.Record, []openapi.InvalidEventProblem) {
	fields, ok := objectFields(rawEvent)
	if !ok {
		return event.Record{}, []openapi.InvalidEventProblem{{Index: index, Field: "$", Code: openapi.Invalid}}
	}
	problems := make([]openapi.InvalidEventProblem, 0)
	checkEventKeys(fields, index, &problems)
	if len(problems) > 0 {
		return event.Record{}, problems
	}

	id, _ := requiredString(fields, "id", index, &problems)
	kindValue, kindOK := requiredString(fields, "kind", index, &problems)
	occurredAt, occurredOK := requiredInt64(fields, "occurred_at", index, &problems)
	installationID, installationOK := nullableString(fields, "installation_id", index, &problems)
	platformValue, platformOK := requiredString(fields, "platform", index, &problems)
	appVersion, appVersionOK := nullableString(fields, "app_version", index, &problems)
	schemaVersion, schemaOK := requiredInt(fields, "schema_version", index, &problems)
	payloadRaw, payloadOK := requiredRaw(fields, "payload", index, &problems)
	if payloadOK {
		payloadSize := len(payloadRaw)
		if rawSize, ok := rawObjectFieldSize(rawEvent, "payload"); ok {
			payloadSize = rawSize
		}
		if payloadSize > eventMaxPayloadBytes {
			problems = append(problems, invalidEventProblem(index, "payload", "too_large"))
		}
	}
	if len(problems) > 0 {
		return event.Record{}, problems
	}
	kind := event.Kind(kindValue)
	if !knownEventKind(kind) {
		problems = append(problems, invalidEventProblem(index, "kind", "unknown"))
	}
	payload, payloadProblems := decodeEventPayload(kind, payloadRaw, index)
	problems = append(problems, payloadProblems...)
	if len(problems) > 0 || !kindOK || !occurredOK || !installationOK || !platformOK || !appVersionOK || !schemaOK {
		return event.Record{}, problems
	}

	record, domainProblems := event.NormalizeAndValidate(event.Input{
		ID:             id,
		Kind:           kind,
		OccurredAt:     occurredAt,
		UserID:         userID,
		InstallationID: installationID,
		Platform:       event.Platform(platformValue),
		AppVersion:     appVersion,
		SchemaVersion:  schemaVersion,
		Payload:        payload,
	})
	for _, problem := range domainProblems {
		problems = append(problems, invalidEventProblem(index, problem.Field, problem.Code))
	}
	return record, problems
}

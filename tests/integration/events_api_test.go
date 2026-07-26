//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

func eventOccurredAt() time.Time {
	return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
}

func TestEventAPIRequiresAnActiveBearerSession(t *testing.T) {
	scenario := newEventRuntimeScenario(t, "auth")
	assertEventRuntimeStatus(t, "missing bearer", scenario.publicClient, scenario.validBody, http.StatusUnauthorized, openapi.ErrorCodeUnauthenticated)

	unknownClient := newAuthenticatedAPIClient(t, fmt.Sprintf("unknown-%d", scenario.suffix))
	assertEventRuntimeStatus(t, "unknown bearer", unknownClient, scenario.validBody, http.StatusUnauthorized, openapi.ErrorCodeUnauthenticated)

	revokedSession := createAuthUser(t, scenario.publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username: fmt.Sprintf("er-%d", scenario.suffix), Password: "secret-password", DisplayName: "Evento Revogado",
	})
	revokedClient := newAuthenticatedAPIClient(t, revokedSession.Token)
	deleteAuthSession(t, revokedClient)
	assertEventRuntimeStatus(t, "revoked bearer", revokedClient, scenario.validBody, http.StatusUnauthorized, openapi.ErrorCodeUnauthenticated)
}

func TestEventAPIIsIdempotentAndAtomic(t *testing.T) {
	scenario := newEventRuntimeScenario(t, "atomic")
	first := postEventsRuntime(t, scenario.client, scenario.validBody)
	requireStatus(t, "first event", first.StatusCode(), http.StatusOK, first.Body)
	if first.JSON200 == nil || first.JSON200.AcceptedCount != 1 || first.JSON200.DuplicateCount != 0 {
		t.Fatalf("first receipt = %#v, want one accepted event", first.JSON200)
	}

	replay := postEventsRuntime(t, scenario.client, scenario.validBody)
	requireStatus(t, "replayed event", replay.StatusCode(), http.StatusOK, replay.Body)
	if replay.JSON200 == nil || replay.JSON200.AcceptedCount != 0 || replay.JSON200.DuplicateCount != 1 {
		t.Fatalf("replay receipt = %#v, want one duplicate event", replay.JSON200)
	}

	atomicEvent := eventRuntimeEnvelope(scenario.suffix, 7, eventOccurredAt().UnixMilli())
	poisonEvent := eventRuntimeEnvelope(scenario.suffix, 8, eventOccurredAt().UnixMilli())
	poisonEvent["kind"] = "client_defined_kind"
	poisonResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, atomicEvent, poisonEvent))
	requireStatus(t, "atomic poison batch", poisonResponse.StatusCode(), http.StatusBadRequest, poisonResponse.Body)
	requireEventProblem(t, poisonResponse, 1, "kind", openapi.Unknown)

	afterPoison := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, atomicEvent))
	requireStatus(t, "event after poison batch", afterPoison.StatusCode(), http.StatusOK, afterPoison.Body)
	if afterPoison.JSON200 == nil || afterPoison.JSON200.AcceptedCount != 1 {
		t.Fatalf("after-poison receipt = %#v, want one accepted event", afterPoison.JSON200)
	}
}

func TestEventAPIRejectsSpoofedAndInvalidEvents(t *testing.T) {
	scenario := newEventRuntimeScenario(t, "validation")
	spoofed := eventRuntimeEnvelope(scenario.suffix, 2, eventOccurredAt().UnixMilli())
	spoofed["user_id"] = eventRuntimeID(scenario.suffix, 999)
	spoofedResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, spoofed))
	requireStatus(t, "spoofed user", spoofedResponse.StatusCode(), http.StatusBadRequest, spoofedResponse.Body)
	requireEventProblem(t, spoofedResponse, 0, "user_id", openapi.Unknown)

	unknownKind := eventRuntimeEnvelope(scenario.suffix, 3, eventOccurredAt().UnixMilli())
	unknownKind["kind"] = "client_defined_kind"
	unknownResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, unknownKind))
	requireStatus(t, "unknown kind", unknownResponse.StatusCode(), http.StatusBadRequest, unknownResponse.Body)
	requireEventProblem(t, unknownResponse, 0, "kind", openapi.Unknown)

	futureSchema := eventRuntimeEnvelope(scenario.suffix, 4, eventOccurredAt().UnixMilli())
	futureSchema["schema_version"] = 2
	futureResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, futureSchema))
	requireStatus(t, "future schema", futureResponse.StatusCode(), http.StatusBadRequest, futureResponse.Body)
	requireEventProblem(t, futureResponse, 0, "schema_version", openapi.Unsupported)

	outOfRange := eventRuntimeEnvelope(scenario.suffix, 5, 0)
	outOfRangeResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, outOfRange))
	requireStatus(t, "out-of-range timestamp", outOfRangeResponse.StatusCode(), http.StatusBadRequest, outOfRangeResponse.Body)
	requireEventProblem(t, outOfRangeResponse, 0, "occurred_at", openapi.Invalid)
}

func TestEventAPIBoundsBatchAndPayloadSizes(t *testing.T) {
	scenario := newEventRuntimeScenario(t, "sizes")
	zeroBatch := postEventsRuntime(t, scenario.client, []byte(`{"events":[]}`))
	requireStatus(t, "empty batch", zeroBatch.StatusCode(), http.StatusBadRequest, zeroBatch.Body)
	requireErrorCode(t, zeroBatch.Body, openapi.ErrorCodeInvalidEventBatch)

	tooMany := make([]map[string]any, 51)
	for index := range tooMany {
		tooMany[index] = eventRuntimeEnvelope(scenario.suffix, index+10, eventOccurredAt().UnixMilli())
	}
	tooManyResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, tooMany...))
	requireStatus(t, "oversized batch", tooManyResponse.StatusCode(), http.StatusBadRequest, tooManyResponse.Body)
	requireErrorCode(t, tooManyResponse.Body, openapi.ErrorCodeInvalidEventBatch)

	largePayload := eventRuntimeSearchEnvelope(scenario.suffix, 6, strings.Repeat("a", 8192))
	largePayloadResponse := postEventsRuntime(t, scenario.client, eventRuntimeRequestBody(t, largePayload))
	requireStatus(t, "oversized payload", largePayloadResponse.StatusCode(), http.StatusBadRequest, largePayloadResponse.Body)
	requireEventProblem(t, largePayloadResponse, 0, "payload", openapi.TooLarge)

	payloadBoundary := eventRuntimeSearchEnvelope(scenario.suffix, 11, "boundary")
	payloadJSON, err := json.Marshal(payloadBoundary["payload"])
	if err != nil {
		t.Fatalf("marshal boundary payload: %v", err)
	}
	payloadBoundaryBody := eventRuntimeRequestBody(t, payloadBoundary)
	payloadMarker := []byte(`"payload":`)
	payloadStart := bytes.Index(payloadBoundaryBody, payloadMarker)
	if payloadStart < 0 {
		t.Fatal("boundary payload marker missing")
	}
	payloadStart += len(payloadMarker)
	payloadBoundaryBody = append(payloadBoundaryBody[:payloadStart], append(bytes.Repeat([]byte(" "), 8*1024-len(payloadJSON)), payloadBoundaryBody[payloadStart:]...)...)
	payloadResponse := postEventsRuntime(t, scenario.client, payloadBoundaryBody)
	requireStatus(t, "maximum payload", payloadResponse.StatusCode(), http.StatusOK, payloadResponse.Body)
	bodyBoundary := append([]byte(nil), scenario.validBody...)
	if len(bodyBoundary) >= 256*1024 {
		t.Fatalf("boundary body fixture is %d bytes, want less than 256 KiB", len(bodyBoundary))
	}
	bodyBoundary = append(bodyBoundary, bytes.Repeat([]byte(" "), 256*1024-len(bodyBoundary))...)
	bodyResponse := postEventsRuntime(t, scenario.client, bodyBoundary)
	requireStatus(t, "maximum body", bodyResponse.StatusCode(), http.StatusOK, bodyResponse.Body)

	overSizedBody := `{"events":[` + strings.Repeat(" ", 256*1024) + `]}`
	overSizedResponse := postEventsRuntime(t, scenario.client, []byte(overSizedBody))
	requireStatus(t, "oversized body", overSizedResponse.StatusCode(), http.StatusRequestEntityTooLarge, overSizedResponse.Body)
	requireErrorCode(t, overSizedResponse.Body, openapi.ErrorCodeRequestTooLarge)
}

func TestEventAPIEnforcesRateLimits(t *testing.T) {
	scenario := newEventRuntimeScenario(t, "rate")
	rateSession := createAuthUser(t, scenario.publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username: fmt.Sprintf("el-%d", scenario.suffix), Password: "secret-password", DisplayName: "Evento Limite",
	})
	rateClient := newAuthenticatedAPIClient(t, rateSession.Token)
	rateRunID := scenario.suffix + 1_000_000
	for batchIndex := range 12 {
		batch := make([]map[string]any, 50)
		for itemIndex := range batch {
			batch[itemIndex] = eventRuntimeEnvelope(rateRunID, batchIndex*50+itemIndex+1, eventOccurredAt().UnixMilli())
		}
		response := postEventsRuntime(t, rateClient, eventRuntimeRequestBody(t, batch...))
		requireStatus(t, "rate-limit fill", response.StatusCode(), http.StatusOK, response.Body)
	}
	rateLimitBatch := make([]map[string]any, 50)
	for itemIndex := range rateLimitBatch {
		rateLimitBatch[itemIndex] = eventRuntimeEnvelope(rateRunID, 601+itemIndex, eventOccurredAt().UnixMilli())
	}
	rateLimited := postEventsRuntime(t, rateClient, eventRuntimeRequestBody(t, rateLimitBatch...))
	requireStatus(t, "rate limit", rateLimited.StatusCode(), http.StatusTooManyRequests, rateLimited.Body)
	retryAfter, err := strconv.Atoi(rateLimited.HTTPResponse.Header.Get("Retry-After"))
	if err != nil || retryAfter < 1 || retryAfter > 60 {
		t.Fatalf("event Retry-After = %q (%v), want integer in [1, 60]", rateLimited.HTTPResponse.Header.Get("Retry-After"), err)
	}
}

type eventRuntimeScenario struct {
	publicClient *openapi.ClientWithResponses
	client       *openapi.ClientWithResponses
	suffix       int64
	validBody    []byte
}

func newEventRuntimeScenario(t *testing.T, name string) eventRuntimeScenario {
	t.Helper()
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)
	suffix := time.Now().UnixNano()
	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username: fmt.Sprintf("ev-%s-%d", name[:1], suffix), Password: "secret-password", DisplayName: "Evento",
	})
	return eventRuntimeScenario{
		publicClient: publicClient,
		client:       newAuthenticatedAPIClient(t, session.Token),
		suffix:       suffix,
		validBody:    eventRuntimeRequestBody(t, eventRuntimeEnvelope(suffix, 1, eventOccurredAt().UnixMilli())),
	}
}

func eventRuntimeEnvelope(runID int64, number int, occurredAt int64) map[string]any {
	return map[string]any{
		"id":              eventRuntimeID(runID, number),
		"kind":            "note_published",
		"occurred_at":     occurredAt,
		"installation_id": eventRuntimeID(runID, 900),
		"platform":        "web",
		"app_version":     "0.0.1",
		"schema_version":  1,
		"payload": map[string]any{
			"note_id":       eventRuntimeID(runID, 100),
			"category_slug": "beauty",
		},
	}
}

func eventRuntimeSearchEnvelope(runID int64, number int, query string) map[string]any {
	return map[string]any{
		"id":              eventRuntimeID(runID, number),
		"kind":            "search_submitted",
		"occurred_at":     eventOccurredAt().UnixMilli(),
		"installation_id": eventRuntimeID(runID, 900),
		"platform":        "web",
		"app_version":     "0.0.1",
		"schema_version":  1,
		"payload": map[string]any{
			"search_id":      eventRuntimeID(runID, 200+number),
			"search_version": "fts5-v1",
			"query":          query,
			"category_slug":  nil,
		},
	}
}

func eventRuntimeID(runID int64, number int) string {
	const idSpace = int64(1_000_000_000_000)
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", (runID%idSpace+int64(number))%idSpace)
}

func eventRuntimeRequestBody(t *testing.T, events ...map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		t.Fatalf("marshal event request: %v", err)
	}
	return body
}

func postEventsRuntime(t *testing.T, client *openapi.ClientWithResponses, body []byte) *openapi.CreateEventsHTTPResponse {
	t.Helper()
	response, err := client.CreateEventsWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/events: %v", err)
	}
	return response
}

func assertEventRuntimeStatus(t *testing.T, name string, client *openapi.ClientWithResponses, body []byte, status int, code openapi.ErrorCode) {
	t.Helper()
	response := postEventsRuntime(t, client, body)
	requireStatus(t, "POST /v1/events "+name, response.StatusCode(), status, response.Body)
	requireErrorCode(t, response.Body, code)
}

func requireEventProblem(t *testing.T, response *openapi.CreateEventsHTTPResponse, index int, field string, code openapi.InvalidEventProblemCode) {
	t.Helper()
	if response.JSON400 == nil {
		t.Fatalf("event response has no JSON400: status=%d body=%s", response.StatusCode(), response.Body)
	}
	result, err := response.JSON400.AsEventErrorResponse()
	if err != nil {
		t.Fatalf("decode indexed event error: %v; body=%s", err, response.Body)
	}
	for _, problem := range result.Problems {
		if problem.Index == index && problem.Field == field && problem.Code == code {
			return
		}
	}
	t.Fatalf("event problems = %+v, want index=%d field=%q code=%q", result.Problems, index, field, code)
}

func requireErrorCode(t *testing.T, body []byte, want openapi.ErrorCode) {
	t.Helper()
	var response openapi.ErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, body)
	}
	if response.Code != want {
		t.Fatalf("error code = %q, want %q", response.Code, want)
	}
}

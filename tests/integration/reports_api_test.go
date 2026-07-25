//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/openapi"
)

func TestReportAPIRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	suffix := time.Now().UnixNano()
	firstSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("r1-%d", suffix),
		Password:    "secret-password",
		DisplayName: "Repórter Um",
	})
	secondSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("r2-%d", suffix),
		Password:    "secret-password",
		DisplayName: "Repórter Dois",
	})
	firstClient := newAuthenticatedAPIClient(t, firstSession.Token)
	secondClient := newAuthenticatedAPIClient(t, secondSession.Token)
	note := createNote(t, firstClient, openapi.CreateNoteJSONRequestBody{
		Title:           "Nota denunciável",
		Body:            "Uma nota para validar denúncias ao vivo.",
		CategorySlug:    "food",
		ClientRequestId: fmt.Sprintf("report-note-%d", suffix),
	})
	comment := createRuntimeComment(t, secondClient, note.Id, "Comentário que será denunciado")

	// First note report from the first reporter: 201 with the exact receipt and
	// no private reporter identifier.
	noteReceipt, noteStatus, noteBody := createRuntimeReport(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.Spam,
	})
	requireStatus(t, "POST /v1/reports note", noteStatus, http.StatusCreated, noteBody)
	requireRuntimeReportReceipt(t, noteReceipt, noteBody, openapi.ReportTargetTypeNote, note.Id, openapi.Spam, nil)
	requireExactReceiptKeys(t, noteBody)

	// First comment report: same 201 contract for the comment target.
	commentReceipt, commentStatus, commentBody := createRuntimeReport(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeComment,
		TargetId:   comment.Id,
		Reason:     openapi.Harassment,
	})
	requireStatus(t, "POST /v1/reports comment", commentStatus, http.StatusCreated, commentBody)
	requireRuntimeReportReceipt(t, commentReceipt, commentBody, openapi.ReportTargetTypeComment, comment.Id, openapi.Harassment, nil)

	// Repeat the note report from the same reporter with a changed reason and
	// explanation: 200, same id and created_at, overwritten reason/details.
	updatedDetails := "contexto adicional da denúncia"
	overwritten, overwriteStatus, overwriteBody := createRuntimeReport(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.HarmfulOrMisleading,
		Details:    &updatedDetails,
	})
	requireStatus(t, "POST /v1/reports overwrite", overwriteStatus, http.StatusOK, overwriteBody)
	if overwritten.Id != noteReceipt.Id {
		t.Fatalf("overwrite id = %q, want original %q", overwritten.Id, noteReceipt.Id)
	}
	if overwritten.CreatedAt != noteReceipt.CreatedAt {
		t.Fatalf("overwrite created_at = %d, want original %d", overwritten.CreatedAt, noteReceipt.CreatedAt)
	}
	if overwritten.Reason != openapi.HarmfulOrMisleading {
		t.Fatalf("overwrite reason = %s, want %s", overwritten.Reason, openapi.HarmfulOrMisleading)
	}
	if overwritten.Details == nil || *overwritten.Details != updatedDetails {
		t.Fatalf("overwrite details = %#v, want %q", overwritten.Details, updatedDetails)
	}
	if bytes.Contains(overwriteBody, []byte(`"reporter_user_id"`)) {
		t.Fatalf("overwrite response exposes reporter_user_id: %s", overwriteBody)
	}

	// A different reporter for the same target gets a separate row: 201, new id.
	secondReporter, secondStatus, secondBody := createRuntimeReport(t, secondClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.Other,
	})
	requireStatus(t, "POST /v1/reports second reporter", secondStatus, http.StatusCreated, secondBody)
	if secondReporter.Id == noteReceipt.Id {
		t.Fatalf("second reporter id = %q, want a distinct id", secondReporter.Id)
	}

	assertRuntimeReportError(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.ReportReason("bogus"),
	}, http.StatusBadRequest, openapi.ErrorCodeInvalidReport, "reason", "invalid")

	assertRuntimeReportError(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   "missing-note",
		Reason:     openapi.Spam,
	}, http.StatusNotFound, openapi.ErrorCodeNotFound, "", "")

	assertRuntimeReportError(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeComment,
		TargetId:   "missing-comment",
		Reason:     openapi.Spam,
	}, http.StatusNotFound, openapi.ErrorCodeNotFound, "", "")

	assertRuntimeReportError(t, publicClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.Spam,
	}, http.StatusUnauthorized, openapi.ErrorCodeUnauthenticated, "", "")

	assertRuntimeReportError(t, firstClient, openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.Spam,
		Details:    strPtr(strings.Repeat("😀", 1001)),
	}, http.StatusBadRequest, openapi.ErrorCodeInvalidReport, "details", "too_long")

	oversized, err := firstClient.CreateReportWithResponse(context.Background(), openapi.CreateReportJSONRequestBody{
		TargetType: openapi.ReportTargetTypeNote,
		TargetId:   note.Id,
		Reason:     openapi.Spam,
		Details:    strPtr(strings.Repeat("x", 8192)),
	})
	if err != nil {
		t.Fatalf("POST /v1/reports oversized body: %v", err)
	}
	requireStatus(t, "POST /v1/reports oversized body", oversized.StatusCode(), http.StatusRequestEntityTooLarge, oversized.Body)
	if oversized.JSON413 == nil || oversized.JSON413.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("oversized report response = %#v, want request_too_large", oversized.JSON413)
	}
}

func createRuntimeReport(t *testing.T, client *openapi.ClientWithResponses, body openapi.CreateReportJSONRequestBody) (openapi.ReportReceipt, int, []byte) {
	t.Helper()

	response, err := client.CreateReportWithResponse(context.Background(), body)
	if err != nil {
		t.Fatalf("POST /v1/reports: %v", err)
	}
	var receipt *openapi.ReportReceipt
	switch response.StatusCode() {
	case http.StatusOK:
		receipt = response.JSON200
	case http.StatusCreated:
		receipt = response.JSON201
	}
	if receipt == nil {
		t.Fatalf("POST /v1/reports returned %d without a receipt; body: %s", response.StatusCode(), response.Body)
	}
	return *receipt, response.StatusCode(), response.Body
}

func requireRuntimeReportReceipt(t *testing.T, receipt openapi.ReportReceipt, body []byte, targetType openapi.ReportTargetType, targetID string, reason openapi.ReportReason, details *string) {
	t.Helper()

	if receipt.TargetType != targetType || receipt.TargetId != targetID || receipt.Reason != reason {
		t.Fatalf("report receipt = %+v, want target %s/%s reason %s", receipt, targetType, targetID, reason)
	}
	if details == nil {
		if receipt.Details != nil {
			t.Fatalf("report details = %#v, want nil", receipt.Details)
		}
	} else if receipt.Details == nil || *receipt.Details != *details {
		t.Fatalf("report details = %#v, want %q", receipt.Details, *details)
	}
	if receipt.Id == "" {
		t.Fatal("report receipt has empty id")
	}
	if receipt.CreatedAt == 0 {
		t.Fatal("report receipt has zero created_at")
	}
	if bytes.Contains(body, []byte(`"reporter_user_id"`)) {
		t.Fatalf("report response exposes reporter_user_id: %s", body)
	}
}

func requireExactReceiptKeys(t *testing.T, body []byte) {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("report body is not a JSON object: %v", err)
	}
	wantKeys := []string{"id", "target_type", "target_id", "reason", "details", "created_at"}
	gotKeys := make([]string, 0, len(raw))
	for key := range raw {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if diff := cmp.Diff(wantKeys, gotKeys); diff != "" {
		t.Fatalf("report receipt keys mismatch (-want +got):\n%s\nbody: %s", diff, body)
	}
}

func assertRuntimeReportError(t *testing.T, client *openapi.ClientWithResponses, body openapi.CreateReportJSONRequestBody, status int, code openapi.ErrorCode, field string, fieldCode string) {
	t.Helper()

	response, err := client.CreateReportWithResponse(context.Background(), body)
	if err != nil {
		t.Fatalf("POST /v1/reports error case: %v", err)
	}
	requireStatus(t, "POST /v1/reports error case", response.StatusCode(), status, response.Body)
	var responseError *openapi.ErrorResponse
	switch status {
	case http.StatusBadRequest:
		responseError = response.JSON400
	case http.StatusUnauthorized:
		responseError = response.JSON401
	case http.StatusNotFound:
		responseError = response.JSON404
	default:
		t.Fatalf("unsupported expected status %d", status)
	}
	if responseError == nil || responseError.Code != code {
		t.Fatalf("report error response = %#v, want %s", responseError, code)
	}
	if field != "" {
		requireRuntimeField(t, *responseError, field, fieldCode)
	}
}

func strPtr(value string) *string {
	return &value
}

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
	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

const exampleReportID = "report-id-1"

func TestCreateReportNoteTargetReturnsCreatedReceipt(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	created := false
	router := newReportRouter(
		fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
			created = true
			want := report.CreateInput{
				TargetType:     report.TargetTypeNote,
				TargetID:       exampleNoteID,
				Reason:         report.ReasonSpam,
				ReporterUserID: "user-id-thiago",
			}
			if input != want {
				t.Fatalf("create input = %#v, want %#v", input, want)
			}
			return report.CreateResult{
				Created: true,
				Report: report.Report{
					ID:         exampleReportID,
					TargetType: input.TargetType,
					TargetID:   input.TargetID,
					Reason:     input.Reason,
					CreatedAt:  createdAt,
				},
			}, nil
		}},
		reportFoundNoteStore(),
		fakeCommentStore{},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"note","target_id":"`+exampleNoteID+`","reason":"spam"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !created {
		t.Fatal("CreateReport was not called")
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	requireExactJSONKeys(t, wire, "id", "target_type", "target_id", "reason", "details", "created_at")
	if wire["id"] != exampleReportID {
		t.Fatalf("id = %#v, want %q", wire["id"], exampleReportID)
	}
	if wire["target_type"] != "note" {
		t.Fatalf("target_type = %#v, want note", wire["target_type"])
	}
	if wire["target_id"] != exampleNoteID {
		t.Fatalf("target_id = %#v, want %q", wire["target_id"], exampleNoteID)
	}
	if wire["reason"] != "spam" {
		t.Fatalf("reason = %#v, want spam", wire["reason"])
	}
	if wire["details"] != nil {
		t.Fatalf("details = %#v, want null", wire["details"])
	}
	requireJSONNumber(t, wire, "created_at", createdAt.UnixMilli())
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestCreateReportCommentTargetReturnsCreatedReceipt(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	created := false
	router := newReportRouter(
		fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
			created = true
			if input.TargetType != report.TargetTypeComment || input.TargetID != exampleCommentID {
				t.Fatalf("create input = %#v, want comment target", input)
			}
			return report.CreateResult{
				Created: true,
				Report: report.Report{
					ID: exampleReportID, TargetType: input.TargetType, TargetID: input.TargetID,
					Reason: input.Reason, CreatedAt: createdAt,
				},
			}, nil
		}},
		fakeNoteStore{},
		fakeCommentStore{findCommentByID: func(_ context.Context, id string) (comment.Comment, error) {
			if id != exampleCommentID {
				t.Fatalf("comment target id = %q, want %q", id, exampleCommentID)
			}
			return comment.Comment{ID: comment.CommentID(id)}, nil
		}},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"comment","target_id":"`+exampleCommentID+`","reason":"harassment"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !created {
		t.Fatal("CreateReport was not called")
	}
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestCreateReportDuplicateReturnsOverwrittenReceipt(t *testing.T) {
	createdAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	router := newReportRouter(
		fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
			return report.CreateResult{
				Created: false,
				Report: report.Report{
					ID: exampleReportID, TargetType: input.TargetType, TargetID: input.TargetID,
					Reason: input.Reason, Details: input.Details, CreatedAt: createdAt,
				},
			}, nil
		}},
		reportFoundNoteStore(),
		fakeCommentStore{},
	)
	details := "Spam repetido"
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"note","target_id":"`+exampleNoteID+`","reason":"other","details":"`+details+`"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	if wire["reason"] != "other" {
		t.Fatalf("reason = %#v, want overwritten other", wire["reason"])
	}
	if wire["details"] != details {
		t.Fatalf("details = %#v, want overwritten %q", wire["details"], details)
	}
	requireNoPrivateWireFields(t, response.Body.String())
}

func TestCreateReportAcceptsAllFourReasons(t *testing.T) {
	reasons := []report.Reason{
		report.ReasonSpam,
		report.ReasonHarassment,
		report.ReasonHarmfulOrMisleading,
		report.ReasonOther,
	}
	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			router := newReportRouter(
				fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
					if input.Reason != reason {
						t.Fatalf("reason = %q, want %q", input.Reason, reason)
					}
					return report.CreateResult{Created: true, Report: report.Report{
						ID: exampleReportID, TargetType: input.TargetType, TargetID: input.TargetID,
						Reason: input.Reason, CreatedAt: time.Now(),
					}}, nil
				}},
				reportFoundNoteStore(),
				fakeCommentStore{},
			)
			request := jsonRequest(http.MethodPost, "/v1/reports",
				`{"target_type":"note","target_id":"`+exampleNoteID+`","reason":"`+string(reason)+`"}`)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d for reason %q", response.Code, http.StatusCreated, reason)
			}
		})
	}
}

func TestCreateReportBlankDetailsAreNull(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "omitted", body: `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam"}`},
		{name: "whitespace", body: `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam","details":"   "}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newReportRouter(
				fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
					if input.Details != nil {
						t.Fatalf("details = %#v, want nil", input.Details)
					}
					return report.CreateResult{Created: true, Report: report.Report{
						ID: exampleReportID, TargetType: input.TargetType, TargetID: input.TargetID,
						Reason: input.Reason, CreatedAt: time.Now(),
					}}, nil
				}},
				reportFoundNoteStore(),
				fakeCommentStore{},
			)
			request := jsonRequest(http.MethodPost, "/v1/reports", test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
			}
			wire := decodeResponseObject(t, response.Body.Bytes())
			if wire["details"] != nil {
				t.Fatalf("details = %#v, want null", wire["details"])
			}
		})
	}
}

func TestCreateReportRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantFields []openapi.ValidationProblem
	}{
		{
			name:       "missing target_type",
			body:       `{"target_id":"` + exampleNoteID + `","reason":"spam"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldTargetType, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "invalid target_type",
			body:       `{"target_type":"place","target_id":"` + exampleNoteID + `","reason":"spam"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldTargetType, Code: openapi.ValidationProblemCodeInvalid}},
		},
		{
			name:       "missing target_id",
			body:       `{"target_type":"note","reason":"spam"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldTargetID, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "missing reason",
			body:       `{"target_type":"note","target_id":"` + exampleNoteID + `"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldReason, Code: openapi.ValidationProblemCodeRequired}},
		},
		{
			name:       "invalid reason",
			body:       `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"unpopular"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldReason, Code: openapi.ValidationProblemCodeInvalid}},
		},
		{
			name:       "over-limit details",
			body:       `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam","details":"` + strings.Repeat("a", report.DetailsMaxLength+1) + `"}`,
			wantFields: []openapi.ValidationProblem{{Field: openapi.ValidationFieldDetails, Code: openapi.ValidationProblemCodeTooLong}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newReportRouter(
				fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
					t.Fatal("CreateReport should not be called")
					return report.CreateResult{}, nil
				}},
				reportFoundNoteStore(),
				fakeCommentStore{},
			)
			request := jsonRequest(http.MethodPost, "/v1/reports", test.body)
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
			if body.Code != openapi.ErrorCodeInvalidReport {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidReport)
			}
			requireValidationProblems(t, body.Fields, test.wantFields)
		})
	}
}
func TestCreateReportRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"reason":`},
		{name: "unknown field", body: `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam","extra":true}`},
		{name: "wrong target_id type", body: `{"target_type":"note","target_id":42,"reason":"spam"}`},
		{name: "trailing JSON", body: `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam"} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newReportRouter(
				fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
					t.Fatal("CreateReport should not be called")
					return report.CreateResult{}, nil
				}},
				reportFoundNoteStore(),
				fakeCommentStore{},
			)
			request := jsonRequest(http.MethodPost, "/v1/reports", test.body)
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

func TestCreateReportEnforcesExactRequestSizeLimit(t *testing.T) {
	reportJSON := []byte(`{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam"}`)
	validBody := append(reportJSON, bytes.Repeat([]byte(" "), int(maxCreateReportRequestBytes)-len(reportJSON))...)
	if int64(len(validBody)) != maxCreateReportRequestBytes {
		t.Fatalf("valid body length = %d, want %d", len(validBody), maxCreateReportRequestBytes)
	}
	created := 0
	router := newReportRouter(
		fakeReportStore{createReport: func(_ context.Context, input report.CreateInput) (report.CreateResult, error) {
			created++
			return report.CreateResult{Created: true, Report: report.Report{
				ID: exampleReportID, TargetType: input.TargetType, TargetID: input.TargetID,
				Reason: input.Reason, CreatedAt: time.Now(),
			}}, nil
		}},
		reportFoundNoteStore(),
		fakeCommentStore{},
	)
	boundaryRequest := httptest.NewRequest(http.MethodPost, "/v1/reports", bytes.NewReader(validBody))
	boundaryRequest.Header.Set("Content-Type", "application/json")
	boundaryResponse := httptest.NewRecorder()

	router.ServeHTTP(boundaryResponse, boundaryRequest)

	requireOpenAPIResponse(t, boundaryRequest, boundaryResponse)
	if boundaryResponse.Code != http.StatusCreated {
		t.Fatalf("exact boundary status = %d, want %d", boundaryResponse.Code, http.StatusCreated)
	}
	if created != 1 {
		t.Fatalf("create calls at boundary = %d, want 1", created)
	}

	oversized := append(validBody, ' ')
	request := httptest.NewRequest(http.MethodPost, "/v1/reports", bytes.NewReader(oversized))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

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

func TestCreateReportMissingNoteTargetReturnsNotFound(t *testing.T) {
	inserted := false
	router := newReportRouter(
		fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
			inserted = true
			return report.CreateResult{}, nil
		}},
		fakeNoteStore{findNote: func(_ context.Context, id string, _ user.UserID) (note.Note, error) {
			return note.Note{}, note.ErrNoteNotFound
		}},
		fakeCommentStore{},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"note","target_id":"missing","reason":"spam"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
	if inserted {
		t.Fatal("CreateReport should not be called for a missing note target")
	}
}

func TestCreateReportMissingCommentTargetReturnsNotFound(t *testing.T) {
	inserted := false
	router := newReportRouter(
		fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
			inserted = true
			return report.CreateResult{}, nil
		}},
		fakeNoteStore{},
		fakeCommentStore{findCommentByID: func(context.Context, string) (comment.Comment, error) {
			return comment.Comment{}, comment.ErrCommentNotFound
		}},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"comment","target_id":"missing","reason":"spam"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	requireErrorCode(t, response, openapi.ErrorCodeNotFound)
	if inserted {
		t.Fatal("CreateReport should not be called for a missing comment target")
	}
}

func TestCreateReportRejectsUnauthenticatedBeforeValidation(t *testing.T) {
	router := NewRouter(
		NotesDependencies{Stores: reportFoundNoteStore(), Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
			t.Fatal("CreateReport should not be called")
			return report.CreateResult{}, nil
		}}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: authenticatedFakeUserStore(fakeUserStore{}), Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports", `{"reason":`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, response, openapi.ErrorCodeUnauthenticated)
}

func TestCreateReportStoreFailureReturnsInternalError(t *testing.T) {
	router := newReportRouter(
		fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
			return report.CreateResult{}, errors.New("database unavailable")
		}},
		reportFoundNoteStore(),
		fakeCommentStore{},
	)
	request := jsonRequest(http.MethodPost, "/v1/reports",
		`{"target_type":"note","target_id":"`+exampleNoteID+`","reason":"spam"}`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	requireErrorCode(t, response, openapi.ErrorCodeInternal)
	if strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("response leaked store error: %s", response.Body.String())
	}
}

func TestCreateReportOwnerStoreFailureReturnsInternalError(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		notes  fakeNoteStore
		others fakeCommentStore
	}{
		{
			name: "note lookup failure",
			body: `{"target_type":"note","target_id":"` + exampleNoteID + `","reason":"spam"}`,
			notes: fakeNoteStore{findNote: func(context.Context, string, user.UserID) (note.Note, error) {
				return note.Note{}, errors.New("database unavailable")
			}},
			others: fakeCommentStore{},
		},
		{
			name:  "comment lookup failure",
			body:  `{"target_type":"comment","target_id":"` + exampleCommentID + `","reason":"spam"}`,
			notes: fakeNoteStore{},
			others: fakeCommentStore{findCommentByID: func(context.Context, string) (comment.Comment, error) {
				return comment.Comment{}, errors.New("database unavailable")
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newReportRouter(
				fakeReportStore{createReport: func(context.Context, report.CreateInput) (report.CreateResult, error) {
					t.Fatal("CreateReport should not be called on owner failure")
					return report.CreateResult{}, nil
				}},
				test.notes,
				test.others,
			)
			request := jsonRequest(http.MethodPost, "/v1/reports", test.body)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			requireOpenAPIResponse(t, request, response)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
			}
			requireErrorCode(t, response, openapi.ErrorCodeInternal)
			if strings.Contains(response.Body.String(), "database unavailable") {
				t.Fatalf("response leaked owner error: %s", response.Body.String())
			}
		})
	}
}

func newReportRouter(reports fakeReportStore, notes fakeNoteStore, comments fakeCommentStore) http.Handler {
	return withCurrentSessionHeader(NewRouter(
		NotesDependencies{Stores: notes, Catalog: fakeCatalog{}},
		CommentDependencies{Store: comments},
		ReportDependencies{Store: reports, CommentTargets: comments},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: authenticatedFakeUserStore(fakeUserStore{}), Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	))
}

func reportFoundNoteStore() fakeNoteStore {
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

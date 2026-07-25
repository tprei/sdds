package httpapi

import (
	"errors"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/report"
	"github.com/tprei/sdds/services/api/internal/user"
)

const maxCreateReportRequestBytes int64 = 8 * 1024

func (handler server) CreateReport(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	var request openapi.CreateReportRequest
	if !decodeJSONRequest(w, r, maxCreateReportRequestBytes, &request) {
		return
	}

	input := report.NormalizeCreateInput(report.CreateInput{
		TargetType:     report.TargetType(request.TargetType),
		TargetID:       request.TargetId,
		Reason:         report.Reason(request.Reason),
		Details:        request.Details,
		ReporterUserID: current.User.ID,
	})
	if problems := report.ValidateCreateInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, reportValidationErrorResponse(problems))
		return
	}

	switch input.TargetType {
	case report.TargetTypeNote:
		if !handler.findReportNote(w, r, input.TargetID, current.User.ID) {
			return
		}
	case report.TargetTypeComment:
		if !handler.findReportComment(w, r, input.TargetID) {
			return
		}
	}

	result, err := handler.reports.store.CreateReport(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, newReportResponse(result.Report))
}

func (handler server) findReportNote(w http.ResponseWriter, r *http.Request, targetID string, viewerUserID user.UserID) bool {
	_, err := handler.reports.notes.FindNote(r.Context(), targetID, viewerUserID)
	if errors.Is(err, note.ErrNoteNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return false
	}
	return true
}

func (handler server) findReportComment(w http.ResponseWriter, r *http.Request, targetID string) bool {
	_, err := handler.reports.comments.FindCommentByID(r.Context(), targetID)
	if errors.Is(err, comment.ErrCommentNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return false
	}
	return true
}

func newReportResponse(found report.Report) openapi.ReportReceipt {
	var details *string
	if found.Details != nil {
		trimmed := *found.Details
		details = &trimmed
	}
	return openapi.ReportReceipt{
		Id:         string(found.ID),
		TargetType: openapi.ReportTargetType(found.TargetType),
		TargetId:   found.TargetID,
		Reason:     openapi.ReportReason(found.Reason),
		Details:    details,
		CreatedAt:  found.CreatedAt.UTC().UnixMilli(),
	}
}

func reportValidationErrorResponse(problems []report.ValidationProblem) openapi.ErrorResponse {
	fields := make([]openapi.ValidationProblem, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, openapi.ValidationProblem{
			Field: openapi.ValidationField(problem.Field),
			Code:  openapi.ValidationProblemCode(problem.Code),
		})
	}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidReport, Fields: &fields}
}

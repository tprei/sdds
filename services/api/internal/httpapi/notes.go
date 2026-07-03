package httpapi

import (
	"net/http"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const recentNotesLimit = 50
const maxCreateNoteRequestBytes int64 = 32 * 1024

func (handler server) ListNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := handler.notes.ListRecentNotes(r.Context(), recentNotesLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	response := openapi.ListNotesResponse{Notes: make([]openapi.Note, 0, len(notes))}
	for _, found := range notes {
		response.Notes = append(response.Notes, newNoteResponse(found))
	}

	writeJSON(w, http.StatusOK, response)
}

func (handler server) CreateNote(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreateNoteRequest
	if !decodeJSONRequest(w, r, maxCreateNoteRequestBytes, &request) {
		return
	}

	input := createNoteInput(request)
	if problems := note.ValidateCreateInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(problems))
		return
	}

	created, err := handler.notes.CreateNote(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusCreated, newNoteResponse(created))
}

func validationErrorResponse(problems []note.ValidationProblem) openapi.ErrorResponse {
	fields := make([]openapi.ValidationProblem, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, openapi.ValidationProblem{
			Field: openapi.ValidationField(problem.Field),
			Code:  openapi.ValidationProblemCode(problem.Message),
		})
	}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidNote, Fields: &fields}
}

func createNoteInput(request openapi.CreateNoteRequest) note.CreateInput {
	return note.NormalizeCreateInput(note.CreateInput{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: note.CategorySlug(request.CategorySlug),
		CitySlug:     note.CitySlug(request.CitySlug),
	})
}

func newNoteResponse(found note.Note) openapi.Note {
	return openapi.Note{
		Id:           found.ID,
		Title:        found.Title,
		Body:         found.Body,
		CategorySlug: openapi.CategorySlug(found.CategorySlug),
		CitySlug:     openapi.CitySlug(found.CitySlug),
		CreatedAt:    found.CreatedAt.UTC().UnixMilli(),
		UpdatedAt:    found.UpdatedAt.UTC().UnixMilli(),
	}
}

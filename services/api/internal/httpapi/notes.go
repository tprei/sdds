package httpapi

import (
	"net/http"

	"github.com/tprei/sdds/services/api/internal/note"
)

const recentNotesLimit = 50
const maxCreateNoteRequestBytes int64 = 32 * 1024

type noteHandler struct {
	notes note.Store
}

type createNoteRequest struct {
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	CategorySlug note.CategorySlug `json:"category_slug"`
	CitySlug     note.CitySlug     `json:"city_slug"`
}

type noteResponse struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	CategorySlug note.CategorySlug `json:"category_slug"`
	CitySlug     note.CitySlug     `json:"city_slug"`
	CreatedAt    int64             `json:"created_at"`
	UpdatedAt    int64             `json:"updated_at"`
}

type listNotesResponse struct {
	Notes []noteResponse `json:"notes"`
}

type validationProblemResponse struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func (handler noteHandler) listNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := handler.notes.ListRecentNotes(r.Context(), recentNotesLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errorResponse{Code: errorCodeInternal})
		return
	}

	response := listNotesResponse{Notes: make([]noteResponse, 0, len(notes))}
	for _, found := range notes {
		response.Notes = append(response.Notes, newNoteResponse(found))
	}

	writeJSON(w, http.StatusOK, response)
}

func (handler noteHandler) createNote(w http.ResponseWriter, r *http.Request) {
	var request createNoteRequest
	if !decodeJSONRequest(w, r, maxCreateNoteRequestBytes, &request) {
		return
	}

	input := request.input()
	if problems := note.ValidateCreateInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(problems))
		return
	}

	created, err := handler.notes.CreateNote(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errorResponse{Code: errorCodeInternal})
		return
	}

	writeJSON(w, http.StatusCreated, newNoteResponse(created))
}

func validationErrorResponse(problems []note.ValidationProblem) errorResponse {
	fields := make([]validationProblemResponse, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, validationProblemResponse{
			Field: problem.Field,
			Code:  problem.Message,
		})
	}
	return errorResponse{Code: errorCodeInvalidNote, Fields: fields}
}

func (request createNoteRequest) input() note.CreateInput {
	return note.NormalizeCreateInput(note.CreateInput{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: request.CategorySlug,
		CitySlug:     request.CitySlug,
	})
}

func newNoteResponse(found note.Note) noteResponse {
	return noteResponse{
		ID:           found.ID,
		Title:        found.Title,
		Body:         found.Body,
		CategorySlug: found.CategorySlug,
		CitySlug:     found.CitySlug,
		CreatedAt:    found.CreatedAt.UTC().UnixMilli(),
		UpdatedAt:    found.UpdatedAt.UTC().UnixMilli(),
	}
}

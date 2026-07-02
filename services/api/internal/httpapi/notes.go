package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

const recentNotesLimit = 50
const maxCreateNoteRequestBytes = 32 * 1024

type noteHandler struct {
	notes NoteStore
}

type createNoteRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Category string `json:"category"`
	City     string `json:"city"`
}

type noteResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Category  string `json:"category"`
	City      string `json:"city"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type listNotesResponse struct {
	Notes []noteResponse `json:"notes"`
}

type validationProblemResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error  string                      `json:"error"`
	Fields []validationProblemResponse `json:"fields,omitempty"`
}

func (handler noteHandler) listNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := handler.notes.ListRecentNotes(r.Context(), recentNotesLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}

	response := listNotesResponse{Notes: make([]noteResponse, 0, len(notes))}
	for _, found := range notes {
		response.Notes = append(response.Notes, mapNoteResponse(found))
	}

	writeJSON(w, http.StatusOK, response)
}

func (handler noteHandler) createNote(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeCreateNoteRequest(w, r)
	if !ok {
		return
	}

	input := note.NormalizeCreateInput(note.CreateInput{
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: note.CategorySlug(request.Category),
		CitySlug:     note.CitySlug(request.City),
	})
	if problems := note.ValidateCreateInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(problems))
		return
	}

	created, err := handler.notes.CreateNote(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}

	writeJSON(w, http.StatusCreated, mapNoteResponse(created))
}

func decodeCreateNoteRequest(w http.ResponseWriter, r *http.Request) (createNoteRequest, bool) {
	var request createNoteRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreateNoteRequestBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeDecodeError(w, err)
		return createNoteRequest{}, false
	}

	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeDecodeError(w, err)
		return createNoteRequest{}, false
	}

	return request, true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request_too_large"})
		return
	}

	writeError(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
}

func validationErrorResponse(problems []note.ValidationProblem) errorResponse {
	fields := make([]validationProblemResponse, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, validationProblemResponse{
			Field:   problem.Field,
			Message: problem.Message,
		})
	}
	return errorResponse{Error: "invalid_note", Fields: fields}
}

func mapNoteResponse(found note.Note) noteResponse {
	return noteResponse{
		ID:        found.ID,
		Title:     found.Title,
		Body:      found.Body,
		Category:  string(found.CategorySlug),
		City:      string(found.CitySlug),
		CreatedAt: formatResponseTime(found.CreatedAt),
		UpdatedAt: formatResponseTime(found.UpdatedAt),
	}
}

func formatResponseTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, body errorResponse) {
	writeJSON(w, status, body)
}

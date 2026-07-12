package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/pagination"
)

const maxAuthorNotesCursorLength = pagination.MaxCursorLength

type authorNotesCursorPayload struct {
	Version   int   `json:"v"`
	CreatedAt int64 `json:"created_at"`
	RowID     int64 `json:"row_id"`
}

func (handler server) GetAuthor(w http.ResponseWriter, r *http.Request, authorID string) {
	if handler.publicAuthors == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	profile, err := handler.publicAuthors.FindPublicAuthor(r.Context(), author.AuthorID(authorID))
	if errors.Is(err, author.ErrAuthorNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newPublicAuthorResponse(profile))
}

func (handler server) ListAuthorNotes(w http.ResponseWriter, r *http.Request, authorID string, params openapi.ListAuthorNotesParams) {
	if handler.publicAuthors == nil || handler.authorNotes == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	cursor, problems := decodeAuthorNotesCursor(params.Cursor)
	input := note.NormalizeAuthorNotesInput(authorNotesInput(authorID, params, cursor))
	problems = append(problems, note.ValidateAuthorNotesInput(input)...)
	if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}

	if _, err := handler.publicAuthors.FindPublicAuthor(r.Context(), author.AuthorID(authorID)); errors.Is(err, author.ErrAuthorNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	page, err := handler.authorNotes.ListAuthorNotes(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	response, err := newAuthorNotesPageResponse(page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func authorNotesInput(authorID string, params openapi.ListAuthorNotesParams, cursor *note.AuthorNotePosition) note.AuthorNotesInput {
	var limit int
	if params.Limit != nil {
		limit = *params.Limit
	}
	return note.AuthorNotesInput{
		AuthorID: author.AuthorID(authorID),
		Limit:    limit,
		After:    cursor,
	}
}

func decodeAuthorNotesCursor(encoded *string) (*note.AuthorNotePosition, []note.ValidationProblem) {
	if encoded == nil {
		return nil, nil
	}
	var payload authorNotesCursorPayload
	if err := pagination.Decode(*encoded, &payload); err != nil {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}
	if payload.Version != 1 || payload.CreatedAt <= 0 || payload.RowID <= 0 {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}
	return &note.AuthorNotePosition{CreatedAt: time.UnixMilli(payload.CreatedAt).UTC(), RowID: payload.RowID}, nil
}

func newPublicAuthorResponse(profile author.PublicAuthor) openapi.PublicAuthor {
	return openapi.PublicAuthor{
		Id:          string(profile.ID),
		DisplayName: profile.DisplayName,
		NoteCount:   profile.NoteCount,
	}
}

func newAuthorNotesPageResponse(page note.AuthorNotesPage) (openapi.AuthorNotesPage, error) {
	response := openapi.AuthorNotesPage{
		Notes:      make([]openapi.Note, 0, len(page.Notes)),
		NextCursor: nil,
	}
	for _, found := range page.Notes {
		response.Notes = append(response.Notes, newNoteResponse(found.Note))
	}
	if page.HasMore && len(page.Notes) > 0 {
		last := page.Notes[len(page.Notes)-1]
		encoded, err := encodeAuthorNotesCursor(last.Position)
		if err != nil {
			return openapi.AuthorNotesPage{}, err
		}
		response.NextCursor = &encoded
	}
	return response, nil
}

func encodeAuthorNotesCursor(cursor note.AuthorNotePosition) (string, error) {
	createdAt := cursor.CreatedAt.UTC().UnixMilli()
	if createdAt <= 0 {
		return "", fmt.Errorf("encode author notes cursor: non-positive created_at")
	}
	if cursor.RowID <= 0 {
		return "", fmt.Errorf("encode author notes cursor: non-positive row_id")
	}
	payload := authorNotesCursorPayload{Version: 1, CreatedAt: createdAt, RowID: cursor.RowID}
	encoded, err := pagination.Encode(payload)
	if err != nil {
		return "", fmt.Errorf("encode author notes cursor: %w", err)
	}
	return encoded, nil
}

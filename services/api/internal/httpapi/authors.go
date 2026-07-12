package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const maxAuthorNotesCursorLength = 160

type authorNotesCursorPayload struct {
	Version   int    `json:"v"`
	CreatedAt int64  `json:"created_at"`
	ID        string `json:"id"`
}

func (handler server) GetAuthor(w http.ResponseWriter, r *http.Request, authorID string) {
	if handler.publicAuthors == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	author, err := handler.publicAuthors.FindPublicAuthor(r.Context(), user.AuthorID(authorID))
	if errors.Is(err, user.ErrAuthorNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newPublicAuthorResponse(author))
}

func (handler server) ListAuthorNotes(w http.ResponseWriter, r *http.Request, authorID string, params openapi.ListAuthorNotesParams) {
	if handler.publicAuthors == nil || handler.authorNotes == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	cursor, problems := decodeAuthorNotesCursor(params.Cursor)
	input := authorNotesInput(authorID, params, cursor)
	problems = append(problems, note.ValidateAuthorNotesInput(input)...)
	if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}

	if _, err := handler.publicAuthors.FindPublicAuthor(r.Context(), user.AuthorID(authorID)); errors.Is(err, user.ErrAuthorNotFound) {
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
	limit := note.AuthorNotesDefaultLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	return note.AuthorNotesInput{
		AuthorID: user.AuthorID(authorID),
		Limit:    limit,
		After:    cursor,
	}
}

func decodeAuthorNotesCursor(encoded *string) (*note.AuthorNotePosition, []note.ValidationProblem) {
	if encoded == nil {
		return nil, nil
	}
	if len(*encoded) > maxAuthorNotesCursorLength {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}

	decoded, err := base64.RawURLEncoding.DecodeString(*encoded)
	if err != nil {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}

	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var payload authorNotesCursorPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}
	if payload.Version != 1 || payload.CreatedAt <= 0 {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}
	if payload.ID == "" {
		return nil, []note.ValidationProblem{{Field: "cursor", Message: "invalid"}}
	}

	return &note.AuthorNotePosition{CreatedAt: time.UnixMilli(payload.CreatedAt).UTC(), ID: payload.ID}, nil
}

func newPublicAuthorResponse(author user.PublicAuthor) openapi.PublicAuthor {
	return openapi.PublicAuthor{
		Id:          string(author.ID),
		DisplayName: author.DisplayName,
		NoteCount:   author.NoteCount,
	}
}

func newAuthorNotesPageResponse(page note.AuthorNotesPage) (openapi.AuthorNotesPage, error) {
	response := openapi.AuthorNotesPage{
		Notes:      make([]openapi.Note, 0, len(page.Notes)),
		NextCursor: nil,
	}
	for _, found := range page.Notes {
		response.Notes = append(response.Notes, newNoteResponse(found))
	}
	if page.HasMore && len(page.Notes) > 0 {
		last := page.Notes[len(page.Notes)-1]
		encoded, err := encodeAuthorNotesCursor(note.AuthorNotePosition{CreatedAt: last.CreatedAt, ID: last.ID})
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
	if cursor.ID == "" {
		return "", fmt.Errorf("encode author notes cursor: empty note id")
	}
	payload := authorNotesCursorPayload{Version: 1, CreatedAt: createdAt, ID: cursor.ID}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode author notes cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

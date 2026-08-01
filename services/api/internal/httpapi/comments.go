package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/pagination"
	"github.com/tprei/sdds/services/api/internal/user"
)

const maxCreateCommentRequestBytes int64 = 8 * 1024

type commentCursorPayload struct {
	Version int   `json:"v"`
	PageKey int64 `json:"page_key"`
}

func (handler server) ListNoteComments(w http.ResponseWriter, r *http.Request, noteID string, params openapi.ListNoteCommentsParams) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if !handler.findCommentNote(w, r, noteID, current.User.ID) {
		return
	}

	cursor, problems := decodeCommentCursor(params.Cursor)
	input := comment.NormalizeListInput(commentListInput(noteID, params, cursor))
	problems = append(problems, comment.ValidateListInput(input)...)
	if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, commentValidationErrorResponse(problems))
		return
	}

	page, err := handler.comments.store.ListNoteComments(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	response, err := newCommentPageResponse(page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (handler server) CreateNoteComment(w http.ResponseWriter, r *http.Request, noteID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if !handler.findCommentNote(w, r, noteID, current.User.ID) {
		return
	}

	var request openapi.CreateCommentRequest
	if !decodeJSONRequest(w, r, maxCreateCommentRequestBytes, &request) {
		return
	}
	input := comment.NormalizeCreateInput(comment.CreateInput{
		NoteID: noteID,
		UserID: current.User.ID,
		Body:   request.Body,
	})
	if problems := comment.ValidateCreateInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, commentValidationErrorResponse(problems))
		return
	}

	created, err := handler.comments.store.CreateComment(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusCreated, newCommentResponse(created))
}

func (handler server) DeleteNoteComment(w http.ResponseWriter, r *http.Request, noteID string, commentID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if !handler.findCommentNote(w, r, noteID, current.User.ID) {
		return
	}

	found, err := handler.comments.store.FindComment(r.Context(), noteID, commentID)
	if errors.Is(err, comment.ErrCommentNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	if found.UserID != current.User.ID {
		writeError(w, http.StatusForbidden, openapi.ErrorResponse{Code: openapi.ErrorCodeForbidden})
		return
	}
	if err := handler.comments.store.DeleteComment(r.Context(), string(found.ID)); errors.Is(err, comment.ErrCommentNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	noContent(w, r)
}

func (handler server) CreateCommentReply(w http.ResponseWriter, r *http.Request, commentID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if handler.comments.store == nil || handler.comments.notes == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	parent, err := handler.comments.store.FindCommentByID(r.Context(), commentID)
	if errors.Is(err, comment.ErrCommentNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	if parent.ParentCommentID != "" {
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidReplyTarget})
		return
	}
	if !handler.findCommentNote(w, r, parent.NoteID, current.User.ID) {
		return
	}
	var request openapi.CreateCommentRequest
	if !decodeJSONRequest(w, r, maxCreateCommentRequestBytes, &request) {
		return
	}
	input := comment.NormalizeCreateReplyInput(comment.CreateReplyInput{
		ParentCommentID: comment.CommentID(commentID),
		UserID:          current.User.ID,
		Body:            request.Body,
	})
	if problems := comment.ValidateCreateReplyInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, commentValidationErrorResponse(problems))
		return
	}
	created, err := handler.comments.store.CreateReply(r.Context(), input)
	if errors.Is(err, comment.ErrCommentNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if errors.Is(err, comment.ErrParentCommentNotTopLevel) {
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidReplyTarget})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusCreated, newCommentResponse(created))
}

func (handler server) findCommentNote(w http.ResponseWriter, r *http.Request, noteID string, viewerUserID user.UserID) bool {
	if handler.comments.store == nil || handler.comments.notes == nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return false
	}
	_, err := handler.comments.notes.FindNote(r.Context(), noteID, viewerUserID)
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

func commentListInput(noteID string, params openapi.ListNoteCommentsParams, cursor *comment.Position) comment.ListInput {
	var limit int
	if params.Limit != nil {
		limit = *params.Limit
	}
	return comment.ListInput{NoteID: noteID, Limit: limit, After: cursor}
}

func decodeCommentCursor(encoded *string) (*comment.Position, []comment.ValidationProblem) {
	if encoded == nil {
		return nil, nil
	}
	var payload commentCursorPayload
	if err := pagination.Decode(*encoded, &payload); err != nil || payload.Version != 1 || payload.PageKey <= 0 {
		return nil, []comment.ValidationProblem{{Field: "cursor", Code: "invalid"}}
	}
	return &comment.Position{PageKey: payload.PageKey}, nil
}

func newCommentResponse(found comment.Comment) openapi.Comment {
	response := openapi.Comment{
		Id:   string(found.ID),
		Body: found.Body,
		Author: openapi.AuthorSummary{
			Id:          string(found.Author.ID),
			DisplayName: found.Author.DisplayName,
		},
		CreatedAt: found.CreatedAt.UTC().UnixMilli(),
	}
	if found.ParentCommentID != "" {
		parentCommentID := string(found.ParentCommentID)
		response.ParentCommentId = &parentCommentID
	}
	return response
}

func newCommentPageResponse(page comment.Page) (openapi.ListNoteCommentsResponse, error) {
	response := openapi.ListNoteCommentsResponse{
		Threads:    make([]openapi.CommentThread, 0, len(page.Comments)),
		NextCursor: nil,
	}
	for _, listed := range page.Comments {
		replies := make([]openapi.Comment, 0, len(listed.Replies))
		for _, reply := range listed.Replies {
			replies = append(replies, newCommentResponse(reply))
		}
		response.Threads = append(response.Threads, openapi.CommentThread{
			Comment:        newCommentResponse(listed.Comment),
			Replies:        replies,
			HasMoreReplies: listed.HasMoreReplies,
		})
	}
	if page.HasMore && len(page.Comments) > 0 {
		encoded, err := encodeCommentCursor(page.Comments[len(page.Comments)-1].Position)
		if err != nil {
			return openapi.ListNoteCommentsResponse{}, err
		}
		response.NextCursor = &encoded
	}
	return response, nil
}

func encodeCommentCursor(position comment.Position) (string, error) {
	if position.PageKey <= 0 {
		return "", fmt.Errorf("encode comment cursor: non-positive page_key")
	}
	encoded, err := pagination.Encode(commentCursorPayload{Version: 1, PageKey: position.PageKey})
	if err != nil {
		return "", fmt.Errorf("encode comment cursor: %w", err)
	}
	return encoded, nil
}

func commentValidationErrorResponse(problems []comment.ValidationProblem) openapi.ErrorResponse {
	fields := make([]openapi.ValidationProblem, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, openapi.ValidationProblem{
			Field: openapi.ValidationField(problem.Field),
			Code:  openapi.ValidationProblemCode(problem.Code),
		})
	}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidComment, Fields: &fields}
}

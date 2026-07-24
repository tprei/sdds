//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

func TestCommentAPIRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	suffix := time.Now().UnixNano()
	firstSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("c1-%d", suffix),
		Password:    "secret-password",
		DisplayName: "Comentador Um",
	})
	secondSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("c2-%d", suffix),
		Password:    "secret-password",
		DisplayName: "Comentadora Dois",
	})
	firstClient := newAuthenticatedAPIClient(t, firstSession.Token)
	secondClient := newAuthenticatedAPIClient(t, secondSession.Token)
	note := createNote(t, firstClient, openapi.CreateNoteJSONRequestBody{
		Title:           "Nota com comentários",
		Body:            "Uma nota para validar comentários ao vivo.",
		CategorySlug:    "food",
		ClientRequestId: fmt.Sprintf("comment-note-%d", suffix),
	})

	first := createRuntimeComment(t, firstClient, note.Id, "Primeiro comentário")
	second := createRuntimeComment(t, secondClient, note.Id, "Segundo comentário")
	third := createRuntimeComment(t, firstClient, note.Id, "Terceiro comentário")
	requireRuntimeComment(t, first, "Primeiro comentário", firstSession.User.Author)
	requireRuntimeComment(t, second, "Segundo comentário", secondSession.User.Author)
	requireRuntimeComment(t, third, "Terceiro comentário", firstSession.User.Author)

	limit := 2
	firstPage := listRuntimeComments(t, firstClient, note.Id, &openapi.ListNoteCommentsParams{Limit: &limit})
	if len(firstPage.Comments) != 2 {
		t.Fatalf("first page comments = %d, want 2", len(firstPage.Comments))
	}
	if firstPage.Comments[0].Id != first.Id || firstPage.Comments[1].Id != second.Id {
		t.Fatalf("first page IDs = [%q, %q], want [%q, %q]", firstPage.Comments[0].Id, firstPage.Comments[1].Id, first.Id, second.Id)
	}
	if firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		t.Fatal("first page next_cursor is nil or empty")
	}

	secondPage := listRuntimeComments(t, firstClient, note.Id, &openapi.ListNoteCommentsParams{Limit: &limit, Cursor: firstPage.NextCursor})
	if len(secondPage.Comments) != 1 || secondPage.Comments[0].Id != third.Id {
		t.Fatalf("second page comments = %#v, want only %q", secondPage.Comments, third.Id)
	}
	if secondPage.NextCursor != nil {
		t.Fatalf("second page next_cursor = %q, want nil", *secondPage.NextCursor)
	}

	assertRuntimeCommentError(t, firstClient, note.Id, openapi.CreateNoteCommentJSONRequestBody{Body: " \t\n "}, http.StatusBadRequest, openapi.ErrorCodeInvalidComment, "body", "required")
	assertRuntimeCommentError(t, firstClient, "missing-note", openapi.CreateNoteCommentJSONRequestBody{Body: "ok"}, http.StatusNotFound, openapi.ErrorCodeNotFound, "", "")
	assertRuntimeCommentError(t, publicClient, note.Id, openapi.CreateNoteCommentJSONRequestBody{Body: "ok"}, http.StatusUnauthorized, openapi.ErrorCodeUnauthenticated, "", "")

	invalidLimit := 51
	invalidList, err := firstClient.ListNoteCommentsWithResponse(context.Background(), note.Id, &openapi.ListNoteCommentsParams{Limit: &invalidLimit})
	if err != nil {
		t.Fatalf("GET /v1/notes/{note_id}/comments invalid limit: %v", err)
	}
	requireStatus(t, "GET /v1/notes/{note_id}/comments invalid limit", invalidList.StatusCode(), http.StatusBadRequest, invalidList.Body)
	if invalidList.JSON400 == nil || invalidList.JSON400.Code != openapi.ErrorCodeInvalidComment {
		t.Fatalf("invalid limit response = %#v, want invalid_comment", invalidList.JSON400)
	}
	requireRuntimeField(t, *invalidList.JSON400, "limit", "invalid")

	validEmoji := createRuntimeComment(t, firstClient, note.Id, strings.Repeat("😀", 1000))
	requireRuntimeComment(t, validEmoji, strings.Repeat("😀", 1000), firstSession.User.Author)
	assertRuntimeCommentError(t, firstClient, note.Id, openapi.CreateNoteCommentJSONRequestBody{Body: strings.Repeat("😀", 1001)}, http.StatusBadRequest, openapi.ErrorCodeInvalidComment, "body", "too_long")

	pageLimit := 50
	beforeOversized := listRuntimeComments(t, firstClient, note.Id, &openapi.ListNoteCommentsParams{Limit: &pageLimit})
	oversized, err := firstClient.CreateNoteCommentWithResponse(context.Background(), note.Id, openapi.CreateNoteCommentJSONRequestBody{Body: strings.Repeat("x", 8192)})
	if err != nil {
		t.Fatalf("POST /v1/notes/{note_id}/comments oversized body: %v", err)
	}
	requireStatus(t, "POST /v1/notes/{note_id}/comments oversized body", oversized.StatusCode(), http.StatusRequestEntityTooLarge, oversized.Body)
	if oversized.JSON413 == nil || oversized.JSON413.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("oversized comment response = %#v, want request_too_large", oversized.JSON413)
	}
	afterOversized := listRuntimeComments(t, firstClient, note.Id, &openapi.ListNoteCommentsParams{Limit: &pageLimit})
	if len(afterOversized.Comments) != len(beforeOversized.Comments) {
		t.Fatalf("oversized comment changed row count: got %d, want %d", len(afterOversized.Comments), len(beforeOversized.Comments))
	}

	injectionBody := `'); DROP TABLE note_comments; --`
	injection := createRuntimeComment(t, firstClient, note.Id, injectionBody)
	requireRuntimeComment(t, injection, injectionBody, firstSession.User.Author)

	unauthenticatedDelete, err := publicClient.DeleteNoteCommentWithResponse(context.Background(), note.Id, first.Id)
	if err != nil {
		t.Fatalf("DELETE /v1/notes/{note_id}/comments/{comment_id} unauthenticated: %v", err)
	}
	requireStatus(t, "DELETE unauthenticated comment", unauthenticatedDelete.StatusCode(), http.StatusUnauthorized, unauthenticatedDelete.Body)
	if unauthenticatedDelete.JSON401 == nil || unauthenticatedDelete.JSON401.Code != openapi.ErrorCodeUnauthenticated {
		t.Fatalf("unauthenticated delete response = %#v, want unauthenticated", unauthenticatedDelete.JSON401)
	}

	injectionPage := listRuntimeComments(t, firstClient, note.Id, &openapi.ListNoteCommentsParams{Limit: &pageLimit})
	if !containsRuntimeComment(injectionPage.Comments, injection.Id, injectionBody) {
		t.Fatalf("SQL-injection comment did not round-trip: %#v", injectionPage.Comments)
	}

	missingDelete, err := firstClient.DeleteNoteCommentWithResponse(context.Background(), note.Id, "missing-comment")
	if err != nil {
		t.Fatalf("DELETE /v1/notes/{note_id}/comments/{comment_id} missing comment: %v", err)
	}
	requireStatus(t, "DELETE missing comment", missingDelete.StatusCode(), http.StatusNotFound, missingDelete.Body)
	if missingDelete.JSON404 == nil || missingDelete.JSON404.Code != openapi.ErrorCodeNotFound {
		t.Fatalf("missing delete response = %#v, want not_found", missingDelete.JSON404)
	}

	forbiddenDelete, err := secondClient.DeleteNoteCommentWithResponse(context.Background(), note.Id, first.Id)
	if err != nil {
		t.Fatalf("DELETE /v1/notes/{note_id}/comments/{comment_id} non-owner: %v", err)
	}
	requireStatus(t, "DELETE non-owner comment", forbiddenDelete.StatusCode(), http.StatusForbidden, forbiddenDelete.Body)
	if forbiddenDelete.JSON403 == nil || forbiddenDelete.JSON403.Code != openapi.ErrorCodeForbidden {
		t.Fatalf("non-owner delete response = %#v, want forbidden", forbiddenDelete.JSON403)
	}

	deleted, err := firstClient.DeleteNoteCommentWithResponse(context.Background(), note.Id, first.Id)
	if err != nil {
		t.Fatalf("DELETE /v1/notes/{note_id}/comments/{comment_id}: %v", err)
	}
	requireStatus(t, "DELETE owned comment", deleted.StatusCode(), http.StatusNoContent, deleted.Body)
}

func createRuntimeComment(t *testing.T, client *openapi.ClientWithResponses, noteID string, body string) openapi.Comment {
	t.Helper()

	response, err := client.CreateNoteCommentWithResponse(context.Background(), noteID, openapi.CreateNoteCommentJSONRequestBody{Body: body})
	if err != nil {
		t.Fatalf("POST /v1/notes/{note_id}/comments: %v", err)
	}
	requireStatus(t, "POST /v1/notes/{note_id}/comments", response.StatusCode(), http.StatusCreated, response.Body)
	if response.JSON201 == nil {
		t.Fatal("POST /v1/notes/{note_id}/comments returned 201 without JSON body")
	}
	if bytes.Contains(response.Body, []byte(`"user_id"`)) || bytes.Contains(response.Body, []byte(`"note_id"`)) {
		t.Fatalf("comment response exposes private identifiers: %s", response.Body)
	}
	return *response.JSON201
}

func listRuntimeComments(t *testing.T, client *openapi.ClientWithResponses, noteID string, params *openapi.ListNoteCommentsParams) openapi.ListNoteCommentsResponse {
	t.Helper()

	response, err := client.ListNoteCommentsWithResponse(context.Background(), noteID, params)
	if err != nil {
		t.Fatalf("GET /v1/notes/{note_id}/comments: %v", err)
	}
	requireStatus(t, "GET /v1/notes/{note_id}/comments", response.StatusCode(), http.StatusOK, response.Body)
	if response.JSON200 == nil {
		t.Fatal("GET /v1/notes/{note_id}/comments returned 200 without JSON body")
	}
	if bytes.Contains(response.Body, []byte(`"user_id"`)) || bytes.Contains(response.Body, []byte(`"note_id"`)) {
		t.Fatalf("comment page exposes private identifiers: %s", response.Body)
	}
	return *response.JSON200
}

func requireRuntimeComment(t *testing.T, comment openapi.Comment, body string, author openapi.AuthorSummary) {
	t.Helper()

	if comment.Id == "" {
		t.Fatal("comment id is empty")
	}
	if comment.Body != body {
		t.Fatalf("comment body = %q, want %q", comment.Body, body)
	}
	if comment.Author != author {
		t.Fatalf("comment author = %#v, want %#v", comment.Author, author)
	}
	if comment.CreatedAt <= 0 {
		t.Fatalf("comment created_at = %d, want positive timestamp", comment.CreatedAt)
	}
}

func containsRuntimeComment(comments []openapi.Comment, id string, body string) bool {
	for _, comment := range comments {
		if comment.Id == id && comment.Body == body {
			return true
		}
	}
	return false
}

func assertRuntimeCommentError(t *testing.T, client *openapi.ClientWithResponses, noteID string, body openapi.CreateNoteCommentJSONRequestBody, status int, code openapi.ErrorCode, field string, fieldCode string) {
	t.Helper()

	response, err := client.CreateNoteCommentWithResponse(context.Background(), noteID, body)
	if err != nil {
		t.Fatalf("POST /v1/notes/{note_id}/comments error case: %v", err)
	}
	requireStatus(t, "POST /v1/notes/{note_id}/comments error case", response.StatusCode(), status, response.Body)
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
		t.Fatalf("error response = %#v, want %s", responseError, code)
	}
	if field != "" {
		requireRuntimeField(t, *responseError, field, fieldCode)
	}
}

func requireRuntimeField(t *testing.T, response openapi.ErrorResponse, field string, code string) {
	t.Helper()

	if response.Fields == nil || len(*response.Fields) != 1 {
		t.Fatalf("error fields = %#v, want one field", response.Fields)
	}
	got := (*response.Fields)[0]
	if string(got.Field) != field || string(got.Code) != code {
		t.Fatalf("error field = %#v, want {%q %q}", got, field, code)
	}
}

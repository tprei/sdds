package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const recentNotesLimit = note.ListDefaultLimit
const searchNotesLimit = note.SearchDefaultLimit
const maxCreateNoteRequestBytes int64 = 32 * 1024

func (handler server) ListNotes(w http.ResponseWriter, r *http.Request, params openapi.ListNotesParams) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	input := listNotesInput(params)
	input.ViewerUserID = current.User.ID
	if problems := note.ValidateListInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}
	if problems, err := handler.validateCategoryFilter(r.Context(), input.CategorySlug); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	} else if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}

	notes, err := handler.notes.store.ListRecentNotes(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListNotesResponse(notes))
}

func (handler server) GetNote(w http.ResponseWriter, r *http.Request, noteID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	found, err := handler.notes.store.FindNote(r.Context(), noteID, current.User.ID)
	if errors.Is(err, note.ErrNoteNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newNoteResponse(found))
}

func (handler server) MarkNoteUseful(w http.ResponseWriter, r *http.Request, noteID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if _, err := handler.notes.store.FindNote(r.Context(), noteID, current.User.ID); err != nil {
		if errors.Is(err, note.ErrNoteNotFound) {
			writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
			return
		}
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	if err := handler.notes.useful.MarkUseful(r.Context(), note.MarkUsefulInput{NoteID: noteID, UserID: current.User.ID}); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	noContent(w, r)
}

func (handler server) UnmarkNoteUseful(w http.ResponseWriter, r *http.Request, noteID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if _, err := handler.notes.store.FindNote(r.Context(), noteID, current.User.ID); err != nil {
		if errors.Is(err, note.ErrNoteNotFound) {
			writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
			return
		}
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	if err := handler.notes.useful.UnmarkUseful(r.Context(), note.UnmarkUsefulInput{NoteID: noteID, UserID: current.User.ID}); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	noContent(w, r)
}

func (handler server) CreateNote(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	var request openapi.CreateNoteRequest
	if !decodeJSONRequest(w, r, maxCreateNoteRequestBytes, &request) {
		return
	}

	input := createNoteInput(request, current.User.ID)
	if problems := note.ValidateCreateInput(input); len(problems) > 0 {
		status := http.StatusBadRequest
		code := openapi.ErrorCodeInvalidNote
		for _, problem := range problems {
			if problem.Field == "image_upload_ids" && problem.Message == "too_long" {
				status = http.StatusConflict
				code = openapi.ErrorCodeTooManyImages
				break
			}
		}
		writeError(w, status, validationErrorResponse(code, problems))
		return
	}

	created, err := handler.notes.store.CreateNote(r.Context(), input)
	if err != nil {
		var catalogValidationErr *note.CatalogValidationError
		if errors.As(err, &catalogValidationErr) {
			writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, catalogValidationErr.ValidationProblems()))
			return
		}
		switch {
		case errors.Is(err, note.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeIdempotencyConflict})
		case errors.Is(err, note.ErrImageUploadExpired):
			writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeUploadExpired})
		case errors.Is(err, note.ErrImageUploadUnavailable):
			writeError(w, http.StatusConflict, validationErrorResponse(openapi.ErrorCodeInvalidNote, []note.ValidationProblem{{Field: "image_upload_ids", Message: "invalid"}}))
		case errors.Is(err, note.ErrCategoryNotFound):
			writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, []note.ValidationProblem{{Field: "category_slug", Message: "unknown"}}))
		case errors.Is(err, note.ErrPlaceNotFound):
			writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, []note.ValidationProblem{{Field: "place_slug", Message: "unknown"}}))
		default:
			writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		}
		return
	}

	writeJSON(w, http.StatusCreated, newNoteResponse(created))
}

func (handler server) SearchNotes(w http.ResponseWriter, r *http.Request, params openapi.SearchNotesParams) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	input := searchNoteInput(params)
	input.ViewerUserID = current.User.ID
	if problems := note.ValidateSearchInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidSearch, problems))
		return
	}
	if problems, err := handler.validateCategoryFilter(r.Context(), input.CategorySlug); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	} else if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidSearch, problems))
		return
	}

	notes, err := handler.notes.store.SearchNotes(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListNotesResponse(notes))
}

func validationErrorResponse(code openapi.ErrorCode, problems []note.ValidationProblem) openapi.ErrorResponse {
	fields := make([]openapi.ValidationProblem, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, openapi.ValidationProblem{
			Field: openapi.ValidationField(problem.Field),
			Code:  openapi.ValidationProblemCode(problem.Message),
		})
	}
	return openapi.ErrorResponse{Code: code, Fields: &fields}
}

func listNotesInput(params openapi.ListNotesParams) note.ListInput {
	categorySlug := note.CategorySlug("")
	if params.CategorySlug != nil {
		categorySlug = note.CategorySlug(*params.CategorySlug)
	}
	return note.NormalizeListInput(note.ListInput{
		CategorySlug: categorySlug,
		Limit:        recentNotesLimit,
	})
}

func createNoteInput(request openapi.CreateNoteRequest, userID user.UserID) note.CreateInput {
	placeSlug := note.PlaceSlug("")
	if request.PlaceSlug != nil {
		placeSlug = note.PlaceSlug(*request.PlaceSlug)
	}
	imageUploadIDs := []string(nil)
	if request.ImageUploadIds != nil {
		imageUploadIDs = *request.ImageUploadIds
	}
	return note.NormalizeCreateInput(note.CreateInput{
		UserID:          userID,
		Title:           request.Title,
		Body:            request.Body,
		CategorySlug:    note.CategorySlug(request.CategorySlug),
		PlaceSlug:       placeSlug,
		ClientRequestID: request.ClientRequestId,
		ImageUploadIDs:  imageUploadIDs,
	})
}

func (handler server) validateCategoryFilter(ctx context.Context, slug note.CategorySlug) ([]note.ValidationProblem, error) {
	if slug == "" {
		return nil, nil
	}

	if _, err := handler.notes.catalog.FindActiveCategory(ctx, slug); errors.Is(err, note.ErrCategoryNotFound) {
		return []note.ValidationProblem{{Field: "category_slug", Message: "unknown"}}, nil
	} else if err != nil {
		return nil, err
	}

	return nil, nil
}

func searchNoteInput(params openapi.SearchNotesParams) note.SearchInput {
	query := ""
	if params.Q != nil {
		query = *params.Q
	}
	categorySlug := note.CategorySlug("")
	if params.CategorySlug != nil {
		categorySlug = note.CategorySlug(*params.CategorySlug)
	}
	return note.NormalizeSearchInput(note.SearchInput{
		CategorySlug: categorySlug,
		Query:        query,
		Limit:        searchNotesLimit,
	})
}

func newListNotesResponse(notes []note.Note) openapi.ListNotesResponse {
	response := openapi.ListNotesResponse{Notes: make([]openapi.Note, 0, len(notes))}
	for _, found := range notes {
		response.Notes = append(response.Notes, newNoteResponse(found))
	}
	return response
}

func newNoteResponse(found note.Note) openapi.Note {
	placeSlug := openapi.PlaceSlug(found.PlaceSlug)
	placeSlugPointer := &placeSlug
	if found.PlaceSlug == "" {
		placeSlugPointer = nil
	}

	images := make([]openapi.NoteImage, 0, len(found.Images))
	for _, image := range found.Images {
		images = append(images, openapi.NoteImage{
			Id:          image.ID,
			Url:         "/v1/media/images/" + image.ID,
			ContentType: openapi.NoteImageContentType(image.ContentType),
			ByteSize:    image.ByteSize,
			Width:       int32(image.Width),
			Height:      int32(image.Height),
			Position:    int32(image.Position),
			CreatedAt:   image.CreatedAt.UTC().UnixMilli(),
			UpdatedAt:   image.UpdatedAt.UTC().UnixMilli(),
		})
	}

	return openapi.Note{
		Id:           found.ID,
		Title:        found.Title,
		Body:         found.Body,
		CategorySlug: openapi.CategorySlug(found.CategorySlug),
		PlaceSlug:    placeSlugPointer,
		Author: openapi.AuthorSummary{
			Id:          string(found.Author.ID),
			DisplayName: found.Author.DisplayName,
		},
		Images:              images,
		UsefulCount:         found.UsefulCount,
		UsefulByCurrentUser: found.UsefulByCurrentUser,
		CreatedAt:           found.CreatedAt.UTC().UnixMilli(),
		UpdatedAt:           found.UpdatedAt.UTC().UnixMilli(),
	}
}

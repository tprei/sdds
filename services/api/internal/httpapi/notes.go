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
	input := listNotesInput(params)
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

	notes, err := handler.notes.ListRecentNotes(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusOK, newListNotesResponse(notes))
}

func (handler server) GetNote(w http.ResponseWriter, r *http.Request, noteID string) {
	found, err := handler.notes.FindNote(r.Context(), noteID)
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
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}
	if problems, err := handler.validateCreateNoteCatalogs(r.Context(), input); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	} else if len(problems) > 0 {
		writeError(w, http.StatusBadRequest, validationErrorResponse(openapi.ErrorCodeInvalidNote, problems))
		return
	}

	created, err := handler.notes.CreateNote(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusCreated, newNoteResponse(created))
}

func (handler server) SearchNotes(w http.ResponseWriter, r *http.Request, params openapi.SearchNotesParams) {
	input := searchNoteInput(params)
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

	notes, err := handler.notes.SearchNotes(r.Context(), input)
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
	return note.NormalizeCreateInput(note.CreateInput{
		UserID:       userID,
		Title:        request.Title,
		Body:         request.Body,
		CategorySlug: note.CategorySlug(request.CategorySlug),
		PlaceSlug:    placeSlug,
	})
}

func (handler server) validateCreateNoteCatalogs(ctx context.Context, input note.CreateInput) ([]note.ValidationProblem, error) {
	problems := make([]note.ValidationProblem, 0, 2)

	if input.CategorySlug != "" {
		if _, err := handler.catalog.FindActiveCategory(ctx, input.CategorySlug); errors.Is(err, note.ErrCategoryNotFound) {
			problems = append(problems, note.ValidationProblem{Field: "category_slug", Message: "unknown"})
		} else if err != nil {
			return nil, err
		}
	}

	if input.PlaceSlug != "" {
		if _, err := handler.catalog.FindActivePlace(ctx, input.PlaceSlug); errors.Is(err, note.ErrPlaceNotFound) {
			problems = append(problems, note.ValidationProblem{Field: "place_slug", Message: "unknown"})
		} else if err != nil {
			return nil, err
		}
	}

	return problems, nil
}

func (handler server) validateCategoryFilter(ctx context.Context, slug note.CategorySlug) ([]note.ValidationProblem, error) {
	if slug == "" {
		return nil, nil
	}

	if _, err := handler.catalog.FindActiveCategory(ctx, slug); errors.Is(err, note.ErrCategoryNotFound) {
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
		CreatedAt: found.CreatedAt.UTC().UnixMilli(),
		UpdatedAt: found.UpdatedAt.UTC().UnixMilli(),
	}
}

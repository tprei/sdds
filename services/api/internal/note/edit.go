package note

import (
	"context"
	"fmt"
	"strings"

	"github.com/tprei/sdds/services/api/internal/user"
)

// EditValidationError reports merged edit fields that fail the note field
// rules shared with the create path.
type EditValidationError struct {
	Problems []ValidationProblem
}

func (err *EditValidationError) Error() string { return "note edit validation failed" }

// ValidationProblems returns the field problems in a stable order for the
// invalid_note error response.
func (err *EditValidationError) ValidationProblems() []ValidationProblem { return err.Problems }

// EditInput is a partial note edit. A nil field is left unchanged.
type EditInput struct {
	NoteID       string
	UserID       user.UserID
	Title        *string
	Body         *string
	CategorySlug *CategorySlug
}

// UpdateInput is a fully merged, validated note edit plus its fresh embedding.
type UpdateInput struct {
	NoteID       string
	UserID       user.UserID
	Title        string
	Body         string
	CategorySlug CategorySlug
	Embedding    Embedding
}

// EditStore is the narrow persistence seam Editor needs.
type EditStore interface {
	FindNote(ctx context.Context, id string, viewerUserID user.UserID) (Note, error)
	UpdateNote(ctx context.Context, input UpdateInput) (Note, error)
	DeleteNote(ctx context.Context, id string, userID user.UserID) error
}

// Editor embeds an edited note's passage before handing the merged edit to the
// store, so an edit never leaves semantic search stale. An embedder failure
// aborts the edit with ErrEmbeddingUnavailable before the store is touched.
type Editor struct {
	store    EditStore
	embedder Embedder
}

func NewEditor(store EditStore, embedder Embedder) *Editor {
	return &Editor{store: store, embedder: embedder}
}

// Edit loads the note, verifies the caller is its author, merges the supplied
// fields, validates them, re-embeds the passage, and persists the update.
func (e *Editor) Edit(ctx context.Context, input EditInput) (Note, error) {
	found, err := e.store.FindNote(ctx, input.NoteID, input.UserID)
	if err != nil {
		return Note{}, err
	}
	if found.UserID != input.UserID {
		return Note{}, ErrNoteForbidden
	}

	title := found.Title
	if input.Title != nil {
		title = strings.TrimSpace(*input.Title)
	}
	body := found.Body
	if input.Body != nil {
		body = strings.TrimSpace(*input.Body)
	}
	slug := found.CategorySlug
	if input.CategorySlug != nil {
		slug = NormalizeCategorySlug(*input.CategorySlug)
	}

	if problems := ValidateEditedNote(title, body, slug); len(problems) > 0 {
		return Note{}, &EditValidationError{Problems: problems}
	}

	passage := EmbeddingPassage(title, body)
	vectors, err := e.embedder.EmbedPassages(ctx, []string{passage})
	if err != nil {
		return Note{}, fmt.Errorf("edit note: %w: %v", ErrEmbeddingUnavailable, err)
	}
	if len(vectors) != 1 {
		return Note{}, fmt.Errorf("edit note: %w: got %d vectors, want 1", ErrEmbeddingUnavailable, len(vectors))
	}

	return e.store.UpdateNote(ctx, UpdateInput{
		NoteID:       input.NoteID,
		UserID:       input.UserID,
		Title:        title,
		Body:         body,
		CategorySlug: slug,
		Embedding: Embedding{
			ModelID:       EmbeddingModelID,
			ModelRevision: EmbeddingModelRevision,
			Dimension:     len(vectors[0]),
			SourceSHA256:  EmbeddingFingerprint(passage),
			Vector:        vectors[0],
		},
	})
}

// Delete removes the note. The store distinguishes a missing note from a
// forbidden one inside its transaction, so there is no pre-read here.
func (e *Editor) Delete(ctx context.Context, noteID string, userID user.UserID) error {
	return e.store.DeleteNote(ctx, noteID, userID)
}

package note

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/user"
)

type stubEditStore struct {
	findNote   func(ctx context.Context, id string, viewerUserID user.UserID) (Note, error)
	updateNote func(ctx context.Context, input UpdateInput) (Note, error)
	deleteNote func(ctx context.Context, id string, userID user.UserID) error
}

func (stub stubEditStore) FindNote(ctx context.Context, id string, viewerUserID user.UserID) (Note, error) {
	if stub.findNote == nil {
		return Note{}, errors.New("FindNote not implemented")
	}
	return stub.findNote(ctx, id, viewerUserID)
}

func (stub stubEditStore) UpdateNote(ctx context.Context, input UpdateInput) (Note, error) {
	if stub.updateNote == nil {
		return Note{}, errors.New("UpdateNote not implemented")
	}
	return stub.updateNote(ctx, input)
}

func (stub stubEditStore) DeleteNote(ctx context.Context, id string, userID user.UserID) error {
	if stub.deleteNote == nil {
		return errors.New("DeleteNote not implemented")
	}
	return stub.deleteNote(ctx, id, userID)
}

func TestEditorEditRejectsNonAuthor(t *testing.T) {
	updateCalled := false
	store := stubEditStore{
		findNote: func(_ context.Context, _ string, _ user.UserID) (Note, error) {
			return Note{ID: "note-1", UserID: "owner-1"}, nil
		},
		updateNote: func(context.Context, UpdateInput) (Note, error) {
			updateCalled = true
			return Note{}, nil
		},
	}
	editor := NewEditor(store, stubEmbedder{})

	_, err := editor.Edit(context.Background(), EditInput{NoteID: "note-1", UserID: "other-1"})
	if !errors.Is(err, ErrNoteForbidden) {
		t.Fatalf("edit error = %v, want ErrNoteForbidden", err)
	}
	if updateCalled {
		t.Fatal("UpdateNote was called for a non-author")
	}
}

func TestEditorEditPropagatesUnknownNote(t *testing.T) {
	store := stubEditStore{
		findNote: func(context.Context, string, user.UserID) (Note, error) {
			return Note{}, ErrNoteNotFound
		},
	}
	editor := NewEditor(store, stubEmbedder{})

	_, err := editor.Edit(context.Background(), EditInput{NoteID: "missing", UserID: "user-1"})
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("edit error = %v, want ErrNoteNotFound", err)
	}
}

func TestEditorEditMergesAndEmbeds(t *testing.T) {
	var gotUpdate UpdateInput
	store := stubEditStore{
		findNote: func(context.Context, string, user.UserID) (Note, error) {
			return Note{
				ID:           "note-1",
				UserID:       "owner-1",
				Title:        "Título original",
				Body:         "Corpo original",
				CategorySlug: CategorySlugFood,
			}, nil
		},
		updateNote: func(_ context.Context, input UpdateInput) (Note, error) {
			gotUpdate = input
			return Note{ID: input.NoteID, Title: input.Title, Body: input.Body, CategorySlug: input.CategorySlug}, nil
		},
	}
	vector := testUnitVector()
	var gotTexts []string
	embedder := stubEmbedder{embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
		gotTexts = texts
		return [][]float32{vector}, nil
	}}
	editor := NewEditor(store, embedder)

	newBody := "Corpo editado"
	_, err := editor.Edit(context.Background(), EditInput{
		NoteID: "note-1",
		UserID: "owner-1",
		Body:   &newBody,
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}

	// A nil field keeps the stored value; the supplied body is trimmed.
	if gotUpdate.Title != "Título original" {
		t.Fatalf("merged title = %q, want %q", gotUpdate.Title, "Título original")
	}
	if gotUpdate.Body != "Corpo editado" {
		t.Fatalf("merged body = %q, want %q", gotUpdate.Body, "Corpo editado")
	}
	if gotUpdate.CategorySlug != CategorySlugFood {
		t.Fatalf("merged category = %q, want %q", gotUpdate.CategorySlug, CategorySlugFood)
	}
	wantPassage := EmbeddingPassage("Título original", "Corpo editado")
	if diff := cmp.Diff([]string{wantPassage}, gotTexts); diff != "" {
		t.Fatalf("embedded texts mismatch (-want +got):\n%s", diff)
	}
	if gotUpdate.Embedding.SourceSHA256 != EmbeddingFingerprint(wantPassage) {
		t.Fatalf("embedding fingerprint = %q, want %q", gotUpdate.Embedding.SourceSHA256, EmbeddingFingerprint(wantPassage))
	}
	if diff := cmp.Diff(vector, gotUpdate.Embedding.Vector); diff != "" {
		t.Fatalf("embedding vector mismatch (-want +got):\n%s", diff)
	}
}

func TestEditorEditRejectsInvalidTitle(t *testing.T) {
	updateCalled := false
	embedCalled := false
	store := stubEditStore{
		findNote: func(context.Context, string, user.UserID) (Note, error) {
			return Note{ID: "note-1", UserID: "owner-1", Title: "Título", Body: "Corpo", CategorySlug: CategorySlugFood}, nil
		},
		updateNote: func(context.Context, UpdateInput) (Note, error) {
			updateCalled = true
			return Note{}, nil
		},
	}
	embedder := stubEmbedder{embedPassages: func(context.Context, []string) ([][]float32, error) {
		embedCalled = true
		return [][]float32{testUnitVector()}, nil
	}}
	editor := NewEditor(store, embedder)

	short := "ab"
	_, err := editor.Edit(context.Background(), EditInput{NoteID: "note-1", UserID: "owner-1", Title: &short})
	var editErr *EditValidationError
	if !errors.As(err, &editErr) {
		t.Fatalf("edit error = %v, want *EditValidationError", err)
	}
	if len(editErr.Problems) != 1 || editErr.Problems[0].Field != "title" {
		t.Fatalf("edit problems = %+v, want one title problem", editErr.Problems)
	}
	if updateCalled {
		t.Fatal("UpdateNote was called for an invalid edit")
	}
	if embedCalled {
		t.Fatal("embedder was called before validation passed")
	}
}

func TestEditorEditAbortsOnEmbedderError(t *testing.T) {
	updateCalled := false
	store := stubEditStore{
		findNote: func(context.Context, string, user.UserID) (Note, error) {
			return Note{ID: "note-1", UserID: "owner-1", Title: "Título", Body: "Corpo", CategorySlug: CategorySlugFood}, nil
		},
		updateNote: func(context.Context, UpdateInput) (Note, error) {
			updateCalled = true
			return Note{}, nil
		},
	}
	embedder := stubEmbedder{embedPassages: func(context.Context, []string) ([][]float32, error) {
		return nil, errors.New("sidecar unreachable")
	}}
	editor := NewEditor(store, embedder)

	_, err := editor.Edit(context.Background(), EditInput{NoteID: "note-1", UserID: "owner-1"})
	if !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("edit error = %v, want ErrEmbeddingUnavailable", err)
	}
	if updateCalled {
		t.Fatal("UpdateNote was called despite embedder failure")
	}
}

func TestEditorDeletePassesThroughStore(t *testing.T) {
	var gotID string
	var gotUser user.UserID
	store := stubEditStore{
		deleteNote: func(_ context.Context, id string, userID user.UserID) error {
			gotID = id
			gotUser = userID
			return nil
		},
	}
	editor := NewEditor(store, stubEmbedder{})

	if err := editor.Delete(context.Background(), "note-1", "owner-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gotID != "note-1" || gotUser != "owner-1" {
		t.Fatalf("delete args = %q/%q, want note-1/owner-1", gotID, gotUser)
	}
}

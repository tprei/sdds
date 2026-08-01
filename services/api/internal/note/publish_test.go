package note

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/user"
)

type stubEmbedder struct {
	embedPassages func(ctx context.Context, texts []string) ([][]float32, error)
}

func (stub stubEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	return nil, errors.New("unexpected embed query")
}

func (stub stubEmbedder) EmbedPassages(ctx context.Context, texts []string) ([][]float32, error) {
	if stub.embedPassages == nil {
		return nil, errors.New("embed passages not implemented")
	}
	return stub.embedPassages(ctx, texts)
}

type stubPublishStore struct {
	createNote func(ctx context.Context, input CreateInput) (Note, error)
}

func (stub stubPublishStore) CreateNote(ctx context.Context, input CreateInput) (Note, error) {
	if stub.createNote == nil {
		return Note{}, errors.New("create note not implemented")
	}
	return stub.createNote(ctx, input)
}

func testUnitVector() []float32 {
	vector := make([]float32, EmbeddingDimension)
	vector[0] = 1
	return vector
}

func TestPublisherEmbedsPassageBeforeCreating(t *testing.T) {
	var gotEmbedding Embedding
	store := stubPublishStore{createNote: func(_ context.Context, input CreateInput) (Note, error) {
		gotEmbedding = input.Embedding
		return Note{ID: "created-note", Title: input.Title, Body: input.Body}, nil
	}}
	vector := testUnitVector()
	var gotTexts []string
	embedder := stubEmbedder{embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
		gotTexts = texts
		return [][]float32{vector}, nil
	}}

	publisher := NewPublisher(store, embedder)
	created, err := publisher.Publish(context.Background(), CreateInput{
		UserID:          user.UserID("user-1"),
		Title:           "  Café bom  ",
		Body:            "  Wi-Fi estável  ",
		CategorySlug:    CategorySlug("food"),
		ClientRequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if created.ID != "created-note" {
		t.Fatalf("created note id = %q, want created-note", created.ID)
	}

	wantPassage := EmbeddingPassage("Café bom", "Wi-Fi estável")
	if diff := cmp.Diff([]string{wantPassage}, gotTexts); diff != "" {
		t.Fatalf("embedded texts mismatch (-want +got):\n%s", diff)
	}
	if gotEmbedding.ModelID != EmbeddingModelID {
		t.Fatalf("embedding model id = %q, want %q", gotEmbedding.ModelID, EmbeddingModelID)
	}
	if gotEmbedding.ModelRevision != EmbeddingModelRevision {
		t.Fatalf("embedding model revision = %q, want %q", gotEmbedding.ModelRevision, EmbeddingModelRevision)
	}
	if gotEmbedding.Dimension != EmbeddingDimension {
		t.Fatalf("embedding dimension = %d, want %d", gotEmbedding.Dimension, EmbeddingDimension)
	}
	if gotEmbedding.SourceSHA256 != EmbeddingFingerprint(wantPassage) {
		t.Fatalf("embedding fingerprint = %q, want %q", gotEmbedding.SourceSHA256, EmbeddingFingerprint(wantPassage))
	}
	if diff := cmp.Diff(vector, gotEmbedding.Vector); diff != "" {
		t.Fatalf("embedding vector mismatch (-want +got):\n%s", diff)
	}
}

func TestPublisherReturnsEmbeddingUnavailableOnEmbedderError(t *testing.T) {
	embedderErr := errors.New("sidecar unreachable")
	createCalled := false
	store := stubPublishStore{createNote: func(context.Context, CreateInput) (Note, error) {
		createCalled = true
		return Note{}, nil
	}}
	embedder := stubEmbedder{embedPassages: func(context.Context, []string) ([][]float32, error) {
		return nil, embedderErr
	}}

	publisher := NewPublisher(store, embedder)
	_, err := publisher.Publish(context.Background(), CreateInput{Title: "Título", Body: "Corpo"})
	if !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("publish error = %v, want ErrEmbeddingUnavailable", err)
	}
	if createCalled {
		t.Fatal("store.CreateNote was called despite embedder failure")
	}
}

func TestPublisherReturnsEmbeddingUnavailableOnWrongVectorCount(t *testing.T) {
	store := stubPublishStore{createNote: func(context.Context, CreateInput) (Note, error) {
		t.Fatal("store.CreateNote should not be called")
		return Note{}, nil
	}}
	embedder := stubEmbedder{embedPassages: func(context.Context, []string) ([][]float32, error) {
		return [][]float32{}, nil
	}}

	publisher := NewPublisher(store, embedder)
	_, err := publisher.Publish(context.Background(), CreateInput{Title: "Título", Body: "Corpo"})
	if !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("publish error = %v, want ErrEmbeddingUnavailable", err)
	}
}

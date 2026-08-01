package note

import (
	"context"
	"errors"
	"fmt"
)

// ErrEmbeddingUnavailable means the embedding runtime could not produce a
// vector for a note being published. There is no fallback: a note is never
// published without its vector, so this always aborts the publish.
var ErrEmbeddingUnavailable = errors.New("embedding unavailable")

// Embedder embeds search text through the private embedding sidecar.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedPassages(ctx context.Context, texts []string) ([][]float32, error)
}

// PublishStore is the narrow persistence seam Publisher needs: an atomic
// create that stores the note and its embedding together.
type PublishStore interface {
	CreateNote(ctx context.Context, input CreateInput) (Note, error)
}

// Publisher embeds a note's passage before handing it to the store, so a
// published note is never missing from semantic search.
type Publisher struct {
	store    PublishStore
	embedder Embedder
}

func NewPublisher(store PublishStore, embedder Embedder) *Publisher {
	return &Publisher{store: store, embedder: embedder}
}

// Publish normalizes the input, embeds its passage, and creates the note with
// the embedding attached. An embedder failure aborts the publish with
// ErrEmbeddingUnavailable before the store is ever called.
func (p *Publisher) Publish(ctx context.Context, input CreateInput) (Note, error) {
	normalized := NormalizeCreateInput(input)

	passage := EmbeddingPassage(normalized.Title, normalized.Body)
	vectors, err := p.embedder.EmbedPassages(ctx, []string{passage})
	if err != nil {
		return Note{}, fmt.Errorf("publish note: %w: %v", ErrEmbeddingUnavailable, err)
	}
	if len(vectors) != 1 {
		return Note{}, fmt.Errorf("publish note: %w: got %d vectors, want 1", ErrEmbeddingUnavailable, len(vectors))
	}

	normalized.Embedding = Embedding{
		ModelID:       EmbeddingModelID,
		ModelRevision: EmbeddingModelRevision,
		Dimension:     len(vectors[0]),
		SourceSHA256:  EmbeddingFingerprint(passage),
		Vector:        vectors[0],
	}

	created, err := p.store.CreateNote(ctx, normalized)
	if err != nil {
		return Note{}, err
	}
	return created, nil
}

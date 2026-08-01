package note

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type fakeEmbeddingIndexStore struct {
	pages           [][]EmbeddingTarget
	listCalls       [][2]any // afterNoteID, limit
	upserted        []string
	upsertEmbedding func(ctx context.Context, noteID string, embedding Embedding, now time.Time) error
}

func (fake *fakeEmbeddingIndexStore) ListEmbeddingTargets(_ context.Context, afterNoteID string, limit int) ([]EmbeddingTarget, error) {
	fake.listCalls = append(fake.listCalls, [2]any{afterNoteID, limit})
	pageIndex := len(fake.listCalls) - 1
	if pageIndex >= len(fake.pages) {
		return nil, nil
	}
	return fake.pages[pageIndex], nil
}

func (fake *fakeEmbeddingIndexStore) UpsertEmbedding(ctx context.Context, noteID string, embedding Embedding, now time.Time) error {
	fake.upserted = append(fake.upserted, noteID)
	if fake.upsertEmbedding != nil {
		return fake.upsertEmbedding(ctx, noteID, embedding, now)
	}
	return nil
}

func fixedNow() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }

func batchEmbedder(dimension int) Embedder {
	return stubEmbedder{embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
		vectors := make([][]float32, len(texts))
		for i := range vectors {
			vectors[i] = make([]float32, dimension)
			vectors[i][0] = 1
		}
		return vectors, nil
	}}
}

func TestReindexEmbeddingsSkipsAlreadyCurrentNotes(t *testing.T) {
	current := EmbeddingTarget{
		NoteID: "note-1", Title: "Café", Body: "Bom",
		ModelID: EmbeddingModelID, ModelRevision: EmbeddingModelRevision,
	}
	current.SourceSHA256 = EmbeddingFingerprint(EmbeddingPassage(current.Title, current.Body))
	store := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{{current}}}

	result, err := ReindexEmbeddings(context.Background(), store, batchEmbedder(EmbeddingDimension), fixedNow)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if result != (ReindexResult{Scanned: 1, Embedded: 0, Skipped: 1}) {
		t.Fatalf("result = %+v, want {Scanned:1 Embedded:0 Skipped:1}", result)
	}
	if len(store.upserted) != 0 {
		t.Fatalf("upserted = %v, want none", store.upserted)
	}
}

func TestReindexEmbeddingsEmbedsMissingAndStaleNotes(t *testing.T) {
	missing := EmbeddingTarget{NoteID: "note-missing", Title: "Título", Body: "Corpo"}
	stale := EmbeddingTarget{
		NoteID: "note-stale", Title: "Outro", Body: "Texto",
		ModelID: "old-model", ModelRevision: EmbeddingModelRevision, SourceSHA256: "irrelevant",
	}
	store := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{{missing, stale}}}

	result, err := ReindexEmbeddings(context.Background(), store, batchEmbedder(EmbeddingDimension), fixedNow)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if result != (ReindexResult{Scanned: 2, Embedded: 2, Skipped: 0}) {
		t.Fatalf("result = %+v, want {Scanned:2 Embedded:2 Skipped:0}", result)
	}
	if diff := cmp.Diff([]string{"note-missing", "note-stale"}, store.upserted); diff != "" {
		t.Fatalf("upserted mismatch (-want +got):\n%s", diff)
	}
}

func TestReindexEmbeddingsPagesUntilShortPage(t *testing.T) {
	fullPage := make([]EmbeddingTarget, reindexPageSize)
	for i := range fullPage {
		fullPage[i] = EmbeddingTarget{NoteID: string(rune('a' + i)), Title: "T", Body: "B"}
	}
	shortPage := []EmbeddingTarget{{NoteID: "zz", Title: "T", Body: "B"}}
	store := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{fullPage, shortPage}}

	result, err := ReindexEmbeddings(context.Background(), store, batchEmbedder(EmbeddingDimension), fixedNow)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if result.Scanned != reindexPageSize+1 {
		t.Fatalf("scanned = %d, want %d", result.Scanned, reindexPageSize+1)
	}
	if len(store.listCalls) != 2 {
		t.Fatalf("list calls = %d, want 2", len(store.listCalls))
	}
	if store.listCalls[0][0] != "" {
		t.Fatalf("first page cursor = %q, want empty", store.listCalls[0][0])
	}
	if store.listCalls[1][0] != fullPage[len(fullPage)-1].NoteID {
		t.Fatalf("second page cursor = %q, want %q", store.listCalls[1][0], fullPage[len(fullPage)-1].NoteID)
	}
}

func TestReindexEmbeddingsStopsOnEmptyPage(t *testing.T) {
	store := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{{}}}
	result, err := ReindexEmbeddings(context.Background(), store, batchEmbedder(EmbeddingDimension), fixedNow)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if result != (ReindexResult{}) {
		t.Fatalf("result = %+v, want zero value", result)
	}
}

func TestReindexEmbeddingsPropagatesEmbedderError(t *testing.T) {
	embedderErr := errors.New("sidecar down")
	store := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{{{NoteID: "note-1", Title: "T", Body: "B"}}}}
	embedder := stubEmbedder{embedPassages: func(context.Context, []string) ([][]float32, error) {
		return nil, embedderErr
	}}
	_, err := ReindexEmbeddings(context.Background(), store, embedder, fixedNow)
	if !errors.Is(err, embedderErr) {
		t.Fatalf("reindex error = %v, want %v", err, embedderErr)
	}
}

func TestReindexEmbeddingsRunningTwiceEmbedsNothingTheSecondTime(t *testing.T) {
	targets := []EmbeddingTarget{{NoteID: "note-1", Title: "Título", Body: "Corpo"}}
	firstStore := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{targets}}
	embedder := batchEmbedder(EmbeddingDimension)

	first, err := ReindexEmbeddings(context.Background(), firstStore, embedder, fixedNow)
	if err != nil {
		t.Fatalf("first reindex: %v", err)
	}
	if first.Embedded != 1 {
		t.Fatalf("first embedded = %d, want 1", first.Embedded)
	}

	// Simulate the store now reporting the freshly-written provenance, as it
	// would after the first run committed.
	updated := targets[0]
	updated.ModelID = EmbeddingModelID
	updated.ModelRevision = EmbeddingModelRevision
	updated.SourceSHA256 = EmbeddingFingerprint(EmbeddingPassage(updated.Title, updated.Body))
	secondStore := &fakeEmbeddingIndexStore{pages: [][]EmbeddingTarget{{updated}}}

	second, err := ReindexEmbeddings(context.Background(), secondStore, embedder, fixedNow)
	if err != nil {
		t.Fatalf("second reindex: %v", err)
	}
	if second.Embedded != 0 {
		t.Fatalf("second embedded = %d, want 0 (idempotent)", second.Embedded)
	}
	if second.Skipped != 1 {
		t.Fatalf("second skipped = %d, want 1", second.Skipped)
	}
}

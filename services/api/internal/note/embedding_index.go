package note

import (
	"context"
	"time"
)

// EmbeddingTarget is one note's current passage plus whatever embedding
// provenance is already stored for it (empty strings when none exists).
type EmbeddingTarget struct {
	NoteID        string
	Title         string
	Body          string
	ModelID       string
	ModelRevision string
	SourceSHA256  string
}

// EmbeddingIndexStore is the narrow persistence seam ReindexEmbeddings needs:
// paging through every note and upserting its embedding.
type EmbeddingIndexStore interface {
	ListEmbeddingTargets(ctx context.Context, afterNoteID string, limit int) ([]EmbeddingTarget, error)
	UpsertEmbedding(ctx context.Context, noteID string, embedding Embedding, now time.Time) error
}

// ReindexResult summarizes one reindex run.
type ReindexResult struct {
	Scanned  int
	Embedded int
	Skipped  int
}

const reindexPageSize = 32

// ReindexEmbeddings pages through every note and re-embeds any whose stored
// embedding is missing or stale (model id, revision, or source fingerprint
// disagrees with the current values). Running it twice in a row embeds
// nothing the second time -- that is the idempotency contract.
func ReindexEmbeddings(ctx context.Context, store EmbeddingIndexStore, embedder Embedder, now func() time.Time) (ReindexResult, error) {
	var result ReindexResult
	afterNoteID := ""

	for {
		targets, err := store.ListEmbeddingTargets(ctx, afterNoteID, reindexPageSize)
		if err != nil {
			return ReindexResult{}, err
		}
		if len(targets) == 0 {
			return result, nil
		}

		var staleTargets []EmbeddingTarget
		var stalePassages []string
		for _, target := range targets {
			result.Scanned++
			passage := EmbeddingPassage(target.Title, target.Body)
			fingerprint := EmbeddingFingerprint(passage)
			if target.ModelID == EmbeddingModelID && target.ModelRevision == EmbeddingModelRevision && target.SourceSHA256 == fingerprint {
				result.Skipped++
				continue
			}
			staleTargets = append(staleTargets, target)
			stalePassages = append(stalePassages, passage)
		}

		if len(staleTargets) > 0 {
			vectors, err := embedder.EmbedPassages(ctx, stalePassages)
			if err != nil {
				return ReindexResult{}, err
			}
			if len(vectors) != len(staleTargets) {
				return ReindexResult{}, ErrEmbeddingUnavailable
			}
			runAt := now()
			for i, target := range staleTargets {
				embedding := Embedding{
					ModelID:       EmbeddingModelID,
					ModelRevision: EmbeddingModelRevision,
					Dimension:     len(vectors[i]),
					SourceSHA256:  EmbeddingFingerprint(EmbeddingPassage(target.Title, target.Body)),
					Vector:        vectors[i],
				}
				if err := store.UpsertEmbedding(ctx, target.NoteID, embedding, runAt); err != nil {
					return ReindexResult{}, err
				}
				result.Embedded++
			}
		}

		afterNoteID = targets[len(targets)-1].NoteID
		if len(targets) < reindexPageSize {
			return result, nil
		}
	}
}

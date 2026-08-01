package sqlite

import (
	"context"
	"fmt"
	"sort"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	searchSemanticSQL = `
		SELECT note_embeddings.note_id, note_embeddings.dimension, note_embeddings.vector
		FROM note_embeddings
		JOIN notes ON notes.id = note_embeddings.note_id
		WHERE note_embeddings.model_id = ? AND note_embeddings.model_revision = ?
	`
	searchSemanticByCategorySQL = `
		SELECT note_embeddings.note_id, note_embeddings.dimension, note_embeddings.vector
		FROM note_embeddings
		JOIN notes ON notes.id = note_embeddings.note_id
		WHERE note_embeddings.model_id = ? AND note_embeddings.model_revision = ?
			AND notes.category_slug = ?
	`
)

// SearchSemantic runs an exact cosine-similarity scan over every stored
// embedding for the pinned production model. Both the query vector and every
// stored vector are L2-normalized, so cosine similarity is a plain dot
// product. Vectors are decoded one row at a time and never retained past the
// loop, keeping memory bounded by one row rather than the whole table.
func (store *NoteStore) SearchSemantic(ctx context.Context, input note.SemanticSearchInput) ([]note.ScoredNote, error) {
	if len(input.Vector) != note.EmbeddingDimension {
		return nil, fmt.Errorf("search semantic: query vector dimension %d, want %d", len(input.Vector), note.EmbeddingDimension)
	}

	query := searchSemanticSQL
	args := []any{note.EmbeddingModelID, note.EmbeddingModelRevision}
	if input.CategorySlug != "" {
		query = searchSemanticByCategorySQL
		args = []any{note.EmbeddingModelID, note.EmbeddingModelRevision, string(input.CategorySlug)}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query note embeddings: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	scored := make([]note.ScoredNote, 0)
	for rows.Next() {
		var noteID string
		var dimension int
		var vectorBlob []byte
		if err := rows.Scan(&noteID, &dimension, &vectorBlob); err != nil {
			return nil, fmt.Errorf("scan note embedding: %w", err)
		}
		vector, err := decodeVector(vectorBlob, dimension)
		if err != nil {
			return nil, fmt.Errorf("decode note embedding for %s: %w", noteID, err)
		}
		scored = append(scored, note.ScoredNote{NoteID: noteID, Score: dotProduct(input.Vector, vector)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read note embeddings: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close note embedding rows: %w", err)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		// UUIDv7 note ids sort by creation time, so descending = newest
		// first, matching the lexical tie-break (created_at DESC, id DESC).
		return scored[i].NoteID > scored[j].NoteID
	})
	if input.Limit >= 0 && len(scored) > input.Limit {
		scored = scored[:input.Limit]
	}
	return scored, nil
}

func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

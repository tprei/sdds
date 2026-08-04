package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	listEmbeddingTargetsSQL = `
		SELECT notes.id, notes.title, notes.body,
		       COALESCE(note_embeddings.model_id, ''),
		       COALESCE(note_embeddings.model_revision, ''),
		       COALESCE(note_embeddings.source_sha256, '')
		FROM notes
		LEFT JOIN note_embeddings ON note_embeddings.note_id = notes.id
		WHERE notes.id > ?
		ORDER BY notes.id
		LIMIT ?
	`
	upsertNoteEmbeddingSQL = `
		INSERT INTO note_embeddings (note_id, model_id, model_revision, dimension, source_sha256, vector, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (note_id) DO UPDATE SET
			model_id = excluded.model_id,
			model_revision = excluded.model_revision,
			dimension = excluded.dimension,
			source_sha256 = excluded.source_sha256,
			vector = excluded.vector,
			updated_at = excluded.updated_at
	`
)

var _ note.EmbeddingIndexStore = (*NoteStore)(nil)

// ListEmbeddingTargets pages through every note ordered by id, reporting
// whatever embedding provenance (if any) is already stored for it. A note
// with no note_embeddings row yet reports empty provenance strings, which
// note.ReindexEmbeddings treats as always stale.
func (store *NoteStore) ListEmbeddingTargets(ctx context.Context, afterNoteID string, limit int) ([]note.EmbeddingTarget, error) {
	rows, err := store.db.QueryContext(ctx, listEmbeddingTargetsSQL, afterNoteID, limit)
	if err != nil {
		return nil, fmt.Errorf("query embedding targets: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	targets := make([]note.EmbeddingTarget, 0, limit)
	for rows.Next() {
		var target note.EmbeddingTarget
		if err := rows.Scan(&target.NoteID, &target.Title, &target.Body, &target.ModelID, &target.ModelRevision, &target.SourceSHA256); err != nil {
			return nil, fmt.Errorf("scan embedding target: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read embedding targets: %w", err)
	}
	return targets, nil
}

// noteEmbeddingExecer is the ExecContext seam shared by the transactional
// write path and the public store method. *sql.DB and *sql.Tx both satisfy it.
type noteEmbeddingExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// upsertNoteEmbedding writes or replaces a note's embedding row through the
// supplied executor, so a caller inside a transaction can reuse it without
// the single-connection store deadlocking on itself.
func upsertNoteEmbedding(ctx context.Context, exec noteEmbeddingExecer, noteID string, embedding note.Embedding, now time.Time) error {
	if len(embedding.Vector) != note.EmbeddingDimension {
		return fmt.Errorf("upsert note embedding: vector length %d, want %d", len(embedding.Vector), note.EmbeddingDimension)
	}
	if _, err := exec.ExecContext(
		ctx,
		upsertNoteEmbeddingSQL,
		noteID,
		embedding.ModelID,
		embedding.ModelRevision,
		note.EmbeddingDimension,
		embedding.SourceSHA256,
		encodeVector(embedding.Vector),
		unixMillis(now),
		unixMillis(now),
	); err != nil {
		return fmt.Errorf("upsert note embedding: %w", err)
	}
	return nil
}

// UpsertEmbedding writes or replaces a note's embedding row. Used by the
// reindex/backfill command; note creation uses the atomic insert in
// note_create.go instead, since it always writes a fresh row.
func (store *NoteStore) UpsertEmbedding(ctx context.Context, noteID string, embedding note.Embedding, now time.Time) error {
	return upsertNoteEmbedding(ctx, store.db, noteID, embedding, now)
}

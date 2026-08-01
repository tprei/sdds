package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestListEmbeddingTargetsReportsNoteWithoutEmbedding(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "list-targets-missing",
		Title:           "Sem embedding",
		Body:            "Ainda não tem vetor.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM note_embeddings WHERE note_id = ?`, created.ID); err != nil {
		t.Fatalf("delete embedding row: %v", err)
	}

	targets, err := store.ListEmbeddingTargets(ctx, "", 10)
	if err != nil {
		t.Fatalf("list embedding targets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}
	target := targets[0]
	if target.NoteID != created.ID {
		t.Fatalf("target note id = %q, want %q", target.NoteID, created.ID)
	}
	if target.ModelID != "" || target.ModelRevision != "" || target.SourceSHA256 != "" {
		t.Fatalf("target provenance = %+v, want all empty", target)
	}
	if target.Title != created.Title || target.Body != created.Body {
		t.Fatalf("target text = %q/%q, want %q/%q", target.Title, target.Body, created.Title, created.Body)
	}
}

func TestListEmbeddingTargetsReportsExistingProvenance(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	embedding := testEmbedding()
	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "list-targets-current",
		Title:           "Com embedding",
		Body:            "Já tem vetor.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       embedding,
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	targets, err := store.ListEmbeddingTargets(ctx, "", 10)
	if err != nil {
		t.Fatalf("list embedding targets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}
	target := targets[0]
	if target.NoteID != created.ID {
		t.Fatalf("target note id = %q, want %q", target.NoteID, created.ID)
	}
	if target.ModelID != note.EmbeddingModelID {
		t.Fatalf("target model id = %q, want %q", target.ModelID, note.EmbeddingModelID)
	}
	if target.ModelRevision != note.EmbeddingModelRevision {
		t.Fatalf("target model revision = %q, want %q", target.ModelRevision, note.EmbeddingModelRevision)
	}
	if target.SourceSHA256 != embedding.SourceSHA256 {
		t.Fatalf("target fingerprint = %q, want %q", target.SourceSHA256, embedding.SourceSHA256)
	}
}

func TestListEmbeddingTargetsPagesAfterCursor(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	var ids []string
	for i := range 3 {
		created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
			Title:        "Nota de paginação",
			Body:         "Corpo qualquer.",
			CategorySlug: note.CategorySlugFood,
		}))
		if err != nil {
			t.Fatalf("create note %d: %v", i, err)
		}
		ids = append(ids, created.ID)
	}

	firstPage, err := store.ListEmbeddingTargets(ctx, "", 1)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage) != 1 {
		t.Fatalf("first page length = %d, want 1", len(firstPage))
	}
	secondPage, err := store.ListEmbeddingTargets(ctx, firstPage[0].NoteID, 10)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	gotSecondPageIDs := make(map[string]bool, len(secondPage))
	for _, target := range secondPage {
		if target.NoteID == firstPage[0].NoteID {
			t.Fatalf("second page repeated note id %q", target.NoteID)
		}
		gotSecondPageIDs[target.NoteID] = true
	}
	if len(secondPage) != 2 {
		t.Fatalf("second page length = %d, want 2", len(secondPage))
	}
	for _, id := range ids {
		if id == firstPage[0].NoteID {
			continue
		}
		if !gotSecondPageIDs[id] {
			t.Fatalf("second page missing note id %q", id)
		}
	}
}

func TestUpsertEmbeddingReplacesExistingRow(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "upsert-replace",
		Title:           "Original",
		Body:            "Vetor original.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	replacement := note.Embedding{
		ModelID:       note.EmbeddingModelID,
		ModelRevision: note.EmbeddingModelRevision,
		Dimension:     note.EmbeddingDimension,
		SourceSHA256:  note.EmbeddingFingerprint("replacement"),
		Vector:        testEmbeddingVector(),
	}
	if err := store.UpsertEmbedding(ctx, created.ID, replacement, time.Now()); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	var count int
	var sourceSHA256 string
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(source_sha256) FROM note_embeddings WHERE note_id = ?`, created.ID).Scan(&count, &sourceSHA256); err != nil {
		t.Fatalf("query embedding after upsert: %v", err)
	}
	if count != 1 {
		t.Fatalf("embedding row count = %d, want 1", count)
	}
	if sourceSHA256 != replacement.SourceSHA256 {
		t.Fatalf("fingerprint after upsert = %q, want %q", sourceSHA256, replacement.SourceSHA256)
	}
}

func TestUpsertEmbeddingInsertsWhenMissing(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "upsert-insert",
		Title:           "Sem vetor ainda",
		Body:            "Precisa de reindex.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM note_embeddings WHERE note_id = ?`, created.ID); err != nil {
		t.Fatalf("delete embedding row: %v", err)
	}

	embedding := testEmbedding()
	if err := store.UpsertEmbedding(ctx, created.ID, embedding, time.Now()); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_embeddings WHERE note_id = ?`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count embedding rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("embedding row count = %d, want 1", count)
	}
}

func TestUpsertEmbeddingRejectsWrongVectorLength(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "upsert-wrong-length",
		Title:           "Nota",
		Body:            "Corpo.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	bad := note.Embedding{
		ModelID:       note.EmbeddingModelID,
		ModelRevision: note.EmbeddingModelRevision,
		SourceSHA256:  note.EmbeddingFingerprint("bad"),
		Vector:        make([]float32, note.EmbeddingDimension/2),
	}
	if err := store.UpsertEmbedding(ctx, created.ID, bad, time.Now()); err == nil {
		t.Fatal("upsert with wrong vector length error = nil, want an error")
	}
}

func TestEmbeddingIndexCascadeOnNoteDelete(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "cascade-index",
		Title:           "Nota apagada",
		Body:            "Deve remover o vetor junto.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, created.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	targets, err := store.ListEmbeddingTargets(ctx, "", 10)
	if err != nil {
		t.Fatalf("list embedding targets: %v", err)
	}
	if diff := cmp.Diff([]note.EmbeddingTarget{}, targets); diff != "" {
		t.Fatalf("targets after delete mismatch (-want +got):\n%s", diff)
	}
}

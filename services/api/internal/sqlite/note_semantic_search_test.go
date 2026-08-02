package sqlite

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

// testVectorAt builds a unit-length vector with the given weights placed at
// dimensions 0..len(weights)-1 and zero elsewhere, so tests can construct
// vectors at a known cosine similarity to each other.
func testVectorAt(weights ...float32) []float32 {
	vector := make([]float32, note.EmbeddingDimension)
	var sumSquares float64
	for i, w := range weights {
		vector[i] = w
		sumSquares += float64(w) * float64(w)
	}
	norm := float32(math.Sqrt(sumSquares))
	for i := range vector {
		vector[i] /= norm
	}
	return vector
}

func testEmbeddingWithVector(vector []float32) note.Embedding {
	return note.Embedding{
		ModelID:       note.EmbeddingModelID,
		ModelRevision: note.EmbeddingModelRevision,
		Dimension:     note.EmbeddingDimension,
		SourceSHA256:  note.EmbeddingFingerprint("test-fixture"),
		Vector:        vector,
	}
}

func TestSearchSemanticRanksNearestVectorFirst(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	near, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-near",
		Title:           "Perto",
		Body:            "Vetor quase idêntico à consulta.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbeddingWithVector(testVectorAt(1, 0)),
	})
	if err != nil {
		t.Fatalf("create near note: %v", err)
	}
	far, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-far",
		Title:           "Longe",
		Body:            "Vetor ortogonal à consulta.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbeddingWithVector(testVectorAt(0, 1)),
	})
	if err != nil {
		t.Fatalf("create far note: %v", err)
	}

	scored, err := store.SearchSemantic(ctx, note.SemanticSearchInput{
		Vector: testVectorAt(1, 0),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search semantic: %v", err)
	}
	if len(scored) != 2 {
		t.Fatalf("scored count = %d, want 2", len(scored))
	}
	if scored[0].NoteID != near.ID {
		t.Fatalf("first result = %q, want nearest note %q", scored[0].NoteID, near.ID)
	}
	if scored[1].NoteID != far.ID {
		t.Fatalf("second result = %q, want farthest note %q", scored[1].NoteID, far.ID)
	}
	if scored[0].Score <= scored[1].Score {
		t.Fatalf("scores not descending: near=%f far=%f", scored[0].Score, scored[1].Score)
	}
}

func TestSearchSemanticCategoryFilterExcludesNearerNoteInOtherCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	// The nearer note is in "travel"; the farther note is in "food". A
	// food-scoped search must return only the farther, food-category note.
	if _, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-category-near-other",
		Title:           "Viagem parecida",
		Body:            "Vetor quase idêntico, categoria errada.",
		CategorySlug:    note.CategorySlugTravel,
		Embedding:       testEmbeddingWithVector(testVectorAt(1, 0)),
	}); err != nil {
		t.Fatalf("create travel note: %v", err)
	}
	food, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-category-far-match",
		Title:           "Comida diferente",
		Body:            "Vetor distante, categoria certa.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbeddingWithVector(testVectorAt(0, 1)),
	})
	if err != nil {
		t.Fatalf("create food note: %v", err)
	}

	scored, err := store.SearchSemantic(ctx, note.SemanticSearchInput{
		Vector:       testVectorAt(1, 0),
		CategorySlug: note.CategorySlugFood,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("search semantic: %v", err)
	}
	if len(scored) != 1 {
		t.Fatalf("scored count = %d, want 1", len(scored))
	}
	if scored[0].NoteID != food.ID {
		t.Fatalf("result = %q, want food note %q", scored[0].NoteID, food.ID)
	}
}

func TestSearchSemanticRejectsWrongQueryVectorLength(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	_, err := store.SearchSemantic(ctx, note.SemanticSearchInput{
		Vector: make([]float32, note.EmbeddingDimension-1),
		Limit:  10,
	})
	if err == nil {
		t.Fatal("search semantic with wrong query vector length error = nil, want an error")
	}
}

func TestSearchSemanticReturnsEmptySliceWhenNoEmbeddingsStored(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	scored, err := store.SearchSemantic(ctx, note.SemanticSearchInput{
		Vector: testVectorAt(1, 0),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search semantic: %v", err)
	}
	if diff := cmp.Diff([]note.ScoredNote{}, scored); diff != "" {
		t.Fatalf("scored mismatch on empty table (-want +got):\n%s", diff)
	}
}

func TestSearchSemanticRejectsCorruptStoredVector(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-corrupt",
		Title:           "Corrompida",
		Body:            "Vetor será corrompido depois.",
		CategorySlug:    note.CategorySlugFood,
		Embedding:       testEmbeddingWithVector(testVectorAt(1, 0)),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	// The note_embeddings CHECK constraint (length(vector) = dimension * 4)
	// makes this state unreachable through normal writes; disable it for
	// this connection only to simulate on-disk corruption.
	if _, err := db.ExecContext(ctx, `PRAGMA ignore_check_constraints = ON`); err != nil {
		t.Fatalf("disable check constraints: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `PRAGMA ignore_check_constraints = OFF`)
	})
	if _, err := db.ExecContext(ctx, `UPDATE note_embeddings SET dimension = ? WHERE note_id = ?`, note.EmbeddingDimension/2, created.ID); err != nil {
		t.Fatalf("corrupt dimension column: %v", err)
	}

	_, err = store.SearchSemantic(ctx, note.SemanticSearchInput{
		Vector: testVectorAt(1, 0),
		Limit:  10,
	})
	if !errors.Is(err, ErrVectorDimensionMismatch) {
		t.Fatalf("search semantic error = %v, want ErrVectorDimensionMismatch", err)
	}
}

func TestFindNotesByIDPreservesRequestedOrderAndSkipsMissing(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	first, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
		Title: "Primeira", Body: "Corpo um.", CategorySlug: note.CategorySlugFood,
	}))
	if err != nil {
		t.Fatalf("create first note: %v", err)
	}
	second, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
		Title: "Segunda", Body: "Corpo dois.", CategorySlug: note.CategorySlugFood,
	}))
	if err != nil {
		t.Fatalf("create second note: %v", err)
	}

	found, err := store.FindNotesByID(ctx, []string{second.ID, "missing-id", first.ID}, systemNoteOwnerUserID)
	if err != nil {
		t.Fatalf("find notes by id: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("found count = %d, want 2", len(found))
	}
	if found[0].ID != second.ID {
		t.Fatalf("found[0] = %q, want %q (requested order)", found[0].ID, second.ID)
	}
	if found[1].ID != first.ID {
		t.Fatalf("found[1] = %q, want %q (requested order)", found[1].ID, first.ID)
	}
}

func TestFindNotesByIDReturnsEmptySliceForEmptyInput(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	found, err := store.FindNotesByID(ctx, nil, systemNoteOwnerUserID)
	if err != nil {
		t.Fatalf("find notes by id: %v", err)
	}
	if diff := cmp.Diff([]note.Note{}, found); diff != "" {
		t.Fatalf("found mismatch on empty input (-want +got):\n%s", diff)
	}
}

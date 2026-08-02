package sqlite

import (
	"context"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

// seedSemanticBenchDB creates n notes, each with a random unit-length
// embedding, so BenchmarkSearchSemantic* measures the real exact cosine
// KNN scan (SQLite row iteration, decode, dot product) at realistic corpus
// sizes rather than a synthetic in-memory-only loop.
func seedSemanticBenchDB(b *testing.B, n int) *testNoteStore {
	b.Helper()
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		b.Fatalf("open database: %v", err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Fatalf("close database: %v", err)
		}
	})
	if err := ApplyMigrations(ctx, db); err != nil {
		b.Fatalf("apply migrations: %v", err)
	}
	store := newTestNoteStore(db, time.Now)

	rng := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < n; i++ {
		vec := make([]float32, note.EmbeddingDimension)
		var sumSquares float64
		for j := range vec {
			v := rng.Float32()
			vec[j] = v
			sumSquares += float64(v) * float64(v)
		}
		norm := float32(math.Sqrt(sumSquares))
		for j := range vec {
			vec[j] /= norm
		}
		_, err := store.CreateNote(ctx, note.CreateInput{
			Title:           "Bench note",
			Body:            "Corpo qualquer para o benchmark de busca semantica.",
			CategorySlug:    "food",
			ClientRequestID: "bench-" + strconv.Itoa(i),
			Embedding: note.Embedding{
				ModelID:       note.EmbeddingModelID,
				ModelRevision: note.EmbeddingModelRevision,
				Dimension:     note.EmbeddingDimension,
				SourceSHA256:  note.EmbeddingFingerprint("bench-fixture-" + strconv.Itoa(i)),
				Vector:        vec,
			},
		})
		if err != nil {
			b.Fatalf("create note: %v", err)
		}
	}
	return store
}

func runSearchSemanticBenchmark(b *testing.B, corpusSize int) {
	store := seedSemanticBenchDB(b, corpusSize)
	ctx := context.Background()
	query := testEmbeddingVector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.SearchSemantic(ctx, note.SemanticSearchInput{Vector: query, Limit: 20}); err != nil {
			b.Fatalf("search semantic: %v", err)
		}
	}
}

func BenchmarkSearchSemantic1000(b *testing.B)   { runSearchSemanticBenchmark(b, 1000) }
func BenchmarkSearchSemantic10000(b *testing.B)  { runSearchSemanticBenchmark(b, 10000) }
func BenchmarkSearchSemantic100000(b *testing.B) { runSearchSemanticBenchmark(b, 100000) }

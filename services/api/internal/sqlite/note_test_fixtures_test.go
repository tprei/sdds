package sqlite

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	systemNoteOwnerUserID   user.UserID     = "00000000-0000-7000-8000-000000000001"
	systemNoteOwnerAuthorID author.AuthorID = "00000000-0000-7000-8000-000000000002"
)

var testCreateInputSequence atomic.Uint64

// testEmbeddingVector returns a deterministic unit-length 768-float vector so
// tests that don't care about embedding content still satisfy the
// note_embeddings dimension and CHECK constraints.
func testEmbeddingVector() []float32 {
	component := float32(1 / math.Sqrt(float64(note.EmbeddingDimension)))
	vector := make([]float32, note.EmbeddingDimension)
	for i := range vector {
		vector[i] = component
	}
	return vector
}

func testEmbedding() note.Embedding {
	return note.Embedding{
		ModelID:       note.EmbeddingModelID,
		ModelRevision: note.EmbeddingModelRevision,
		Dimension:     note.EmbeddingDimension,
		SourceSHA256:  note.EmbeddingFingerprint("test-fixture"),
		Vector:        testEmbeddingVector(),
	}
}

func testCreateInput(input note.CreateInput) note.CreateInput {
	sequence := testCreateInputSequence.Add(1)
	input.ClientRequestID = "sqlite-test-" + strconv.FormatUint(sequence, 10)
	if len(input.Embedding.Vector) == 0 {
		input.Embedding = testEmbedding()
	}
	return input
}

type testNoteStore struct {
	*NoteStore
}

func newTestNoteStore(db *sql.DB, clock func() time.Time) *testNoteStore {
	if _, err := db.Exec(`
		INSERT INTO users (id, state, created_at, updated_at)
		VALUES (?, 'active', 0, 0)
		ON CONFLICT (id) DO NOTHING`, systemNoteOwnerUserID); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`
		INSERT INTO authors (id, user_id, display_name, created_at, updated_at)
		VALUES (?, ?, 'sdds', 0, 0)
		ON CONFLICT (id) DO NOTHING`, systemNoteOwnerAuthorID, systemNoteOwnerUserID); err != nil {
		panic(err)
	}
	return &testNoteStore{NoteStore: newNoteStore(db, clock)}
}

func (store *testNoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	input.UserID = systemNoteOwnerUserID
	return store.NoteStore.CreateNote(ctx, input)
}

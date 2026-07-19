package sqlite

import (
	"context"
	"database/sql"
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

func testCreateInput(input note.CreateInput) note.CreateInput {
	sequence := testCreateInputSequence.Add(1)
	input.ClientRequestID = "sqlite-test-" + strconv.FormatUint(sequence, 10)
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

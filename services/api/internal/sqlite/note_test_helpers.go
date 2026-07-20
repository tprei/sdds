package sqlite

import (
	"strconv"
	"sync/atomic"

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

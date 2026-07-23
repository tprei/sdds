package note

import (
	"context"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

// UsefulReaction records that a user marked a note as useful.
type UsefulReaction struct {
	NoteID    string
	UserID    user.UserID
	CreatedAt time.Time
}

// MarkUsefulInput captures the note and viewer for a useful mark.
type MarkUsefulInput struct {
	NoteID string
	UserID user.UserID
}

// UnmarkUsefulInput captures the note and viewer for a useful unmark.
type UnmarkUsefulInput struct {
	NoteID string
	UserID user.UserID
}

// UsefulStore persists user-scoped useful reactions.
type UsefulStore interface {
	MarkUseful(context.Context, MarkUsefulInput) error
	UnmarkUseful(context.Context, UnmarkUsefulInput) error
}

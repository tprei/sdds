package comment

import (
	"context"
	"errors"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
)

var ErrCommentNotFound = errors.New("comment not found")

var ErrParentCommentNotTopLevel = errors.New("parent comment is not top level")

type CommentID string

type AuthorSummary struct {
	ID          author.AuthorID
	DisplayName string
}

type Comment struct {
	ID        CommentID
	NoteID    string
	UserID    user.UserID
	Body      string
	Author    AuthorSummary
	CreatedAt time.Time
	// ParentCommentID is empty for a top-level comment and set to the parent's id for a reply.
	ParentCommentID CommentID
}

type CreateInput struct {
	NoteID string
	UserID user.UserID
	Body   string
}

// CreateReplyInput carries a reply's parent and body. The note is derived from
// the parent comment, never from the caller.
type CreateReplyInput struct {
	ParentCommentID CommentID
	UserID          user.UserID
	Body            string
}

type Position struct {
	PageKey int64
}

type ListedComment struct {
	Comment        Comment
	Position       Position
	Replies        []Comment
	HasMoreReplies bool
}

type ListInput struct {
	NoteID string
	Limit  int
	After  *Position
}

type Page struct {
	Comments []ListedComment
	HasMore  bool
}

// Store owns note-scoped comment read and write operations.
type Store interface {
	CreateComment(ctx context.Context, input CreateInput) (Comment, error)
	FindComment(ctx context.Context, noteID, id string) (Comment, error)
	ListNoteComments(ctx context.Context, input ListInput) (Page, error)
	DeleteComment(ctx context.Context, id string) error
	CreateReply(ctx context.Context, input CreateReplyInput) (Comment, error)
	FindCommentByID(ctx context.Context, id string) (Comment, error)
}

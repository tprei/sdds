package comment

import (
	"context"
	"errors"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
)

var ErrCommentNotFound = errors.New("comment not found")

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
}

type CreateInput struct {
	NoteID string
	UserID user.UserID
	Body   string
}

type Position struct {
	PageKey int64
}

type ListedComment struct {
	Comment  Comment
	Position Position
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

type Store interface {
	CreateComment(ctx context.Context, input CreateInput) (Comment, error)
	FindComment(ctx context.Context, noteID, id string) (Comment, error)
	ListNoteComments(ctx context.Context, input ListInput) (Page, error)
	DeleteComment(ctx context.Context, id string) error
}

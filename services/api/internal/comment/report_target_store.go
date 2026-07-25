package comment

import "context"

// ReportTargetStore reads a comment by id for the report domain. It lets report
// intake validate a generic comment id through the comment store boundary
// without widening the existing Store interface with a note-scoped lookup.
type ReportTargetStore interface {
	FindCommentByID(ctx context.Context, id string) (Comment, error)
}

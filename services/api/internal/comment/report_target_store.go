package comment

import "context"

// ReportTargetStore reads a comment by id for the report domain. Report
// intake and reply creation both need a note-less lookup, so Store also
// exposes FindCommentByID; this narrower interface injects only that
// capability into the report handler.
type ReportTargetStore interface {
	FindCommentByID(ctx context.Context, id string) (Comment, error)
}

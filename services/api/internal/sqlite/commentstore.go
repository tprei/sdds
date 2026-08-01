package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	commentProjectionSQL = `
		note_comments.id,
		note_comments.note_id,
		note_comments.user_id,
		note_comments.body,
		authors.id,
		authors.display_name,
		note_comments.created_at,
		note_comments.parent_comment_id
	`
	findCommentSQL = `
		SELECT
		` + commentProjectionSQL + `
		FROM note_comments
		JOIN authors ON authors.user_id = note_comments.user_id
		WHERE note_comments.note_id = ? AND note_comments.id = ?
	`
	findCommentByIDSQL = `
		SELECT
		` + commentProjectionSQL + `
		FROM note_comments
		JOIN authors ON authors.user_id = note_comments.user_id
		WHERE note_comments.id = ?
	`
	listNoteCommentsSQL = `
		SELECT
			note_comments.comment_page_key,
		` + commentProjectionSQL + `
		FROM note_comments
		JOIN authors ON authors.user_id = note_comments.user_id
		WHERE note_comments.note_id = ?
			AND note_comments.parent_comment_id IS NULL
		ORDER BY note_comments.comment_page_key ASC
		LIMIT ?
	`
	listNoteCommentsAfterSQL = `
		SELECT
			note_comments.comment_page_key,
		` + commentProjectionSQL + `
		FROM note_comments
		JOIN authors ON authors.user_id = note_comments.user_id
		WHERE note_comments.note_id = ?
			AND note_comments.comment_page_key > ?
			AND note_comments.parent_comment_id IS NULL
		ORDER BY note_comments.comment_page_key ASC
		LIMIT ?
	`
	listCommentRepliesSQL = `
		SELECT
			note_comments.parent_comment_id,
		` + commentProjectionSQL + `
		FROM (
			SELECT
				inner_comments.*,
				ROW_NUMBER() OVER (
					PARTITION BY inner_comments.parent_comment_id
					ORDER BY inner_comments.comment_page_key
				) AS reply_rank
			FROM note_comments AS inner_comments
			WHERE inner_comments.parent_comment_id IN (%s)
		) AS note_comments
		JOIN authors ON authors.user_id = note_comments.user_id
		WHERE note_comments.reply_rank <= ?
		ORDER BY note_comments.parent_comment_id, note_comments.comment_page_key
	`
)

var _ comment.Store = (*CommentStore)(nil)

type CommentStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewCommentStore(db *sql.DB) *CommentStore {
	return newCommentStore(db, time.Now)
}

func newCommentStore(db *sql.DB, clock func() time.Time) *CommentStore {
	return &CommentStore{db: db, clock: clock}
}

func (store *CommentStore) CreateComment(ctx context.Context, input comment.CreateInput) (comment.Comment, error) {
	normalized := comment.NormalizeCreateInput(input)
	if problems := comment.ValidateCreateInput(normalized); len(problems) > 0 {
		return comment.Comment{}, fmt.Errorf("create comment: invalid input")
	}

	id, err := comment.NewID()
	if err != nil {
		return comment.Comment{}, fmt.Errorf("create comment id: %w", err)
	}
	now := normalizeTime(store.clock())
	if _, err := store.db.ExecContext(
		ctx,
		`INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		string(id),
		normalized.NoteID,
		string(normalized.UserID),
		normalized.Body,
		unixMillis(now),
	); err != nil {
		return comment.Comment{}, fmt.Errorf("insert comment: %w", err)
	}

	created, err := store.FindComment(ctx, normalized.NoteID, string(id))
	if err != nil {
		return comment.Comment{}, fmt.Errorf("load created comment: %w", err)
	}
	return created, nil
}

func (store *CommentStore) FindComment(ctx context.Context, noteID, id string) (comment.Comment, error) {
	found, err := scanCommentRow(store.db.QueryRowContext(ctx, findCommentSQL, noteID, id))
	if err == nil {
		return found, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return comment.Comment{}, comment.ErrCommentNotFound
	}
	return comment.Comment{}, fmt.Errorf("find comment: %w", err)
}

// FindCommentByID looks up a single comment by id without a parent note id. It
// exists for report intake, which receives a generic comment target id.
func (store *CommentStore) FindCommentByID(ctx context.Context, id string) (comment.Comment, error) {
	found, err := scanCommentRow(store.db.QueryRowContext(ctx, findCommentByIDSQL, id))
	if err == nil {
		return found, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return comment.Comment{}, comment.ErrCommentNotFound
	}
	return comment.Comment{}, fmt.Errorf("find comment by id: %w", err)
}

func (store *CommentStore) ListNoteComments(ctx context.Context, input comment.ListInput) (page comment.Page, err error) {
	normalized := comment.NormalizeListInput(input)
	if problems := comment.ValidateListInput(normalized); len(problems) > 0 {
		return comment.Page{}, fmt.Errorf("list note comments: invalid input")
	}

	fetchLimit := normalized.Limit + 1
	query := listNoteCommentsSQL
	args := []any{normalized.NoteID, fetchLimit}
	if normalized.After != nil {
		query = listNoteCommentsAfterSQL
		args = []any{normalized.NoteID, normalized.After.PageKey, fetchLimit}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return comment.Page{}, fmt.Errorf("query note comments: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close note comments rows: %w", closeErr)
		}
	}()

	comments := make([]comment.ListedComment, 0, fetchLimit)
	for rows.Next() {
		found, err := scanListedComment(rows)
		if err != nil {
			return comment.Page{}, err
		}
		comments = append(comments, found)
	}
	if err := rows.Err(); err != nil {
		return comment.Page{}, fmt.Errorf("read note comments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return comment.Page{}, fmt.Errorf("close note comments rows: %w", err)
	}

	page.Comments = comments
	if len(comments) > normalized.Limit {
		page.Comments = comments[:normalized.Limit]
		page.HasMore = true
	}
	if err := store.loadReplies(ctx, page.Comments); err != nil {
		return comment.Page{}, err
	}
	return page, nil
}

func (store *CommentStore) DeleteComment(ctx context.Context, id string) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM note_comments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted comment count: %w", err)
	}
	if deleted == 0 {
		return comment.ErrCommentNotFound
	}
	return nil
}

func scanCommentRow(row *sql.Row) (comment.Comment, error) {
	return scanCommentValues(row.Scan)
}

func scanListedComment(rows *sql.Rows) (comment.ListedComment, error) {
	var pageKey int64
	found, err := scanCommentValues(func(destinations ...any) error {
		destinations = append([]any{&pageKey}, destinations...)
		return rows.Scan(destinations...)
	})
	if err != nil {
		return comment.ListedComment{}, err
	}
	return comment.ListedComment{
		Comment:  found,
		Position: comment.Position{PageKey: pageKey},
	}, nil
}

func scanCommentValues(scan func(...any) error) (comment.Comment, error) {
	var found comment.Comment
	var id string
	var userID string
	var authorID string
	var createdAt int64
	var parentCommentID sql.NullString
	if err := scan(
		&id,
		&found.NoteID,
		&userID,
		&found.Body,
		&authorID,
		&found.Author.DisplayName,
		&createdAt,
		&parentCommentID,
	); err != nil {
		return comment.Comment{}, fmt.Errorf("scan comment: %w", err)
	}

	found.ID = comment.CommentID(id)
	found.UserID = user.UserID(userID)
	found.Author.ID = author.AuthorID(authorID)
	found.CreatedAt = timeFromUnixMillis(createdAt)
	found.ParentCommentID = comment.CommentID(parentCommentID.String)
	return found, nil
}

func (store *CommentStore) loadReplies(ctx context.Context, listed []comment.ListedComment) (err error) {
	if len(listed) == 0 {
		return nil
	}

	parentIDs := make([]string, 0, len(listed))
	for _, entry := range listed {
		parentIDs = append(parentIDs, string(entry.Comment.ID))
	}

	placeholders := strings.Repeat("?, ", len(parentIDs)-1) + "?"
	query := fmt.Sprintf(listCommentRepliesSQL, placeholders)
	args := make([]any, 0, len(parentIDs)+1)
	for _, id := range parentIDs {
		args = append(args, id)
	}
	args = append(args, comment.ReplyMaxPerParent+1)

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query note comment replies: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close note comment replies rows: %w", closeErr)
		}
	}()

	repliesByParent := make(map[comment.CommentID][]comment.Comment)
	for rows.Next() {
		parentID, reply, err := scanReplyRow(rows)
		if err != nil {
			return fmt.Errorf("read note comment replies: %w", err)
		}
		repliesByParent[parentID] = append(repliesByParent[parentID], reply)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read note comment replies: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close note comment replies rows: %w", err)
	}

	for i := range listed {
		replies := repliesByParent[listed[i].Comment.ID]
		if len(replies) > comment.ReplyMaxPerParent {
			listed[i].HasMoreReplies = true
			listed[i].Replies = replies[:comment.ReplyMaxPerParent]
		} else {
			listed[i].Replies = replies
		}
	}
	return nil
}

func scanReplyRow(rows *sql.Rows) (comment.CommentID, comment.Comment, error) {
	var parentCommentID string
	reply, err := scanCommentValues(func(destinations ...any) error {
		destinations = append([]any{&parentCommentID}, destinations...)
		return rows.Scan(destinations...)
	})
	if err != nil {
		return "", comment.Comment{}, err
	}
	return comment.CommentID(parentCommentID), reply, nil
}

func (store *CommentStore) CreateReply(ctx context.Context, input comment.CreateReplyInput) (created comment.Comment, err error) {
	normalized := comment.NormalizeCreateReplyInput(input)
	if problems := comment.ValidateCreateReplyInput(normalized); len(problems) > 0 {
		return comment.Comment{}, fmt.Errorf("create reply: invalid input")
	}

	id, err := comment.NewID()
	if err != nil {
		return comment.Comment{}, fmt.Errorf("create reply id: %w", err)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return comment.Comment{}, fmt.Errorf("begin create reply: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback create reply: %w", rollbackErr)
		}
	}()

	var parentNoteID string
	var parentOfParent sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT note_id, parent_comment_id FROM note_comments WHERE id = ?", string(normalized.ParentCommentID)).Scan(&parentNoteID, &parentOfParent)
	if errors.Is(err, sql.ErrNoRows) {
		return comment.Comment{}, comment.ErrCommentNotFound
	}
	if err != nil {
		return comment.Comment{}, fmt.Errorf("find parent comment: %w", err)
	}
	if parentOfParent.Valid {
		return comment.Comment{}, comment.ErrParentCommentNotTopLevel
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO note_comments (id, note_id, user_id, body, created_at, parent_comment_id) VALUES (?, ?, ?, ?, ?, ?)", string(id), parentNoteID, string(normalized.UserID), normalized.Body, unixMillis(normalizeTime(store.clock())), string(normalized.ParentCommentID)); err != nil {
		return comment.Comment{}, fmt.Errorf("insert reply: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return comment.Comment{}, fmt.Errorf("commit create reply: %w", err)
	}

	created, err = store.FindComment(ctx, parentNoteID, string(id))
	if err != nil {
		return comment.Comment{}, fmt.Errorf("load created reply: %w", err)
	}
	return created, nil
}

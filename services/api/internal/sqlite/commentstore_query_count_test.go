package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
)

const countingSQLiteDriverName = "sdds_comment_counting"

var (
	countingSQLiteDriverOnce sync.Once
	countingSQLiteDriverErr  error
	commentQueryCount        atomic.Int64
)

type countingSQLiteDriver struct {
	base driver.Driver
}

func (driver countingSQLiteDriver) Open(name string) (driver.Conn, error) {
	conn, err := driver.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingSQLiteConn{Conn: conn}, nil
}

type countingSQLiteConn struct {
	driver.Conn
}

func (conn *countingSQLiteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := conn.Conn.(driver.QueryerContext)
	if !ok {
		return nil, fmt.Errorf("sqlite connection does not implement QueryerContext")
	}
	commentQueryCount.Add(1)
	return queryer.QueryContext(ctx, query, args)
}

func (conn *countingSQLiteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := conn.Conn.(driver.ExecerContext)
	if !ok {
		return nil, fmt.Errorf("sqlite connection does not implement ExecerContext")
	}
	return execer.ExecContext(ctx, query, args)
}

func (conn *countingSQLiteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := conn.Conn.(driver.ConnPrepareContext)
	if !ok {
		return conn.Prepare(query)
	}
	return preparer.PrepareContext(ctx, query)
}

func registerCountingSQLiteDriver() {
	countingSQLiteDriverOnce.Do(func() {
		baseDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			countingSQLiteDriverErr = err
			return
		}
		defer func() {
			if closeErr := baseDB.Close(); closeErr != nil && countingSQLiteDriverErr == nil {
				countingSQLiteDriverErr = closeErr
			}
		}()
		sql.Register(countingSQLiteDriverName, countingSQLiteDriver{base: baseDB.Driver()})
	})
}

func openCountingSQLiteDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	registerCountingSQLiteDriver()
	if countingSQLiteDriverErr != nil {
		t.Fatalf("register counting sqlite driver: %v", countingSQLiteDriverErr)
	}
	db, err := sql.Open(countingSQLiteDriverName, t.TempDir()+"/sdds.db")
	if err != nil {
		t.Fatalf("open counting sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close counting sqlite database: %v", err)
		}
	})
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, enableForeignKeysSQL); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := db.ExecContext(ctx, setBusyTimeoutSQL); err != nil {
		t.Fatalf("set busy timeout: %v", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func TestCommentStoreListUsesOneBatchedReplyQuery(t *testing.T) {
	ctx := context.Background()
	db := openCountingSQLiteDatabase(t, ctx)
	store := newCommentStore(db, func() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) })
	insertCommentStoreAuthor(t, ctx, db, commentStoreOwnerUserID, commentStoreOwnerAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, commentStoreNoteID, commentStoreOwnerUserID, 0)

	for parentIndex := range comment.ListMaxLimit {
		parent := createComment(t, ctx, store, commentStoreNoteID, commentStoreOwnerUserID, fmt.Sprintf("parent-%d", parentIndex))
		for replyIndex := range comment.ReplyMaxPerParent + 5 {
			if _, err := store.CreateReply(ctx, comment.CreateReplyInput{
				ParentCommentID: parent.ID,
				UserID:          commentStoreOwnerUserID,
				Body:            fmt.Sprintf("reply-%d-%d", parentIndex, replyIndex),
			}); err != nil {
				t.Fatalf("create reply %d/%d: %v", parentIndex, replyIndex, err)
			}
		}
	}

	commentQueryCount.Store(0)
	page, err := store.ListNoteComments(ctx, comment.ListInput{
		NoteID: commentStoreNoteID,
		Limit:  comment.ListMaxLimit,
	})
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if got := commentQueryCount.Load(); got != 2 {
		t.Fatalf("list query count = %d, want exactly 2 (top-level page plus one batched reply query)", got)
	}
	if len(page.Comments) != comment.ListMaxLimit {
		t.Fatalf("top-level comment count = %d, want %d", len(page.Comments), comment.ListMaxLimit)
	}
	for index, listed := range page.Comments {
		if len(listed.Replies) != comment.ReplyMaxPerParent {
			t.Fatalf("parent %d reply count = %d, want %d", index, len(listed.Replies), comment.ReplyMaxPerParent)
		}
		if !listed.HasMoreReplies {
			t.Fatalf("parent %d HasMoreReplies = false, want true", index)
		}
	}
}

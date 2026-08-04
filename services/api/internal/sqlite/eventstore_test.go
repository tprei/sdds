package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	storeUserID              = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d11"
	storeUserIDTwo           = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d12"
	storeInstallID           = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d13"
	storeEventID             = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d14"
	storeEventIDTwo          = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d15"
	storeSearchID            = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d16"
	storeSearchIDTwo         = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d17"
	storeEventIDComment      = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d19"
	storeEventIDCommentReply = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d20"
	storeCommentID           = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d21"
	storeReplyCommentID      = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d22"
	storeParentCommentID     = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d23"
)

func TestApplyMigrationsCreatesEvents(t *testing.T) {
	db := openMigratedDatabase(t, context.Background())
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'events'`).Scan(&count); err != nil {
		t.Fatalf("query events table: %v", err)
	}
	if count != 1 {
		t.Fatalf("events table count = %d, want 1", count)
	}
}

func TestApplyMigrationsCreatesEventIndexes(t *testing.T) {
	db := openMigratedDatabase(t, context.Background())
	indexes := []string{
		"events_received_page_idx",
		"events_occurred_page_idx",
		"events_kind_occurred_page_idx",
		"events_search_occurred_page_idx",
		"events_user_occurred_page_idx",
		"events_installation_occurred_page_idx",
	}
	for _, index := range indexes {
		t.Run(index, func(t *testing.T) {
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
				t.Fatalf("query index: %v", err)
			}
			if count != 1 {
				t.Fatalf("index count = %d, want 1", count)
			}
		})
	}
}

func TestEventStoreAppendBatchPreservesOrderAndCounts(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	store := NewEventStore(db)
	recordOne := makeEventRecord(t, storeEventID, storeUserID, event.KindSearchSubmitted, event.SearchSubmittedPayload{
		SearchID: storeSearchID, SearchVersion: event.SearchVersionFTS5V1, Query: " café ",
	})
	recordTwo := makeEventRecord(t, storeEventIDTwo, storeUserID, event.KindNotePublished, event.NotePublishedPayload{
		NoteID: storeEventID, CategorySlug: "food",
	})
	receivedAt := time.Date(2026, time.July, 2, 12, 0, 0, 123000000, time.UTC).Add(987 * time.Nanosecond)
	result, err := store.AppendBatch(ctx, []event.Record{recordOne, recordTwo}, receivedAt)
	if err != nil {
		t.Fatalf("append events: %v", err)
	}
	if result.AcceptedCount != 2 || result.DuplicateCount != 0 {
		t.Fatalf("append result = %+v", result)
	}

	rows, err := db.QueryContext(ctx, `SELECT event_page_key, id, occurred_at, received_at, search_id, payload_json FROM events ORDER BY event_page_key`)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	var first struct {
		pageKey, occurredAt, receivedAt int64
		id, searchID, payload           string
	}
	if !rows.Next() {
		t.Fatal("first event missing")
	}
	if err := rows.Scan(&first.pageKey, &first.id, &first.occurredAt, &first.receivedAt, &first.searchID, &first.payload); err != nil {
		t.Fatalf("scan first event: %v", err)
	}
	if first.pageKey != 1 || first.id != storeEventID || first.searchID != storeSearchID || first.receivedAt != receivedAt.UnixMilli() {
		t.Fatalf("first event = %+v", first)
	}
	if !strings.Contains(first.payload, `"query":"café"`) || strings.Contains(first.payload, ` "query"`) {
		t.Fatalf("payload was not compact/normalized: %q", first.payload)
	}
	if !rows.Next() {
		t.Fatal("second event missing")
	}
	var secondPageKey int64
	var secondID string
	var secondSearchID sql.NullString
	if err := rows.Scan(&secondPageKey, &secondID, new(int64), new(int64), &secondSearchID, new(string)); err != nil {
		t.Fatalf("scan second event: %v", err)
	}
	if secondPageKey != 2 || secondID != storeEventIDTwo || secondSearchID.Valid {
		t.Fatalf("second event page/id/search = %d/%s/%+v", secondPageKey, secondID, secondSearchID)
	}
	if rows.Next() {
		t.Fatal("unexpected third event")
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close event rows: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate events: %v", err)
	}
}

func TestEventStoreAppendBatchIDOnlyReplayKeepsFirstRow(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	insertEventTestUser(t, db, storeUserIDTwo)
	store := NewEventStore(db)
	first := makeEventRecord(t, storeEventID, storeUserID, event.KindSearchSubmitted, event.SearchSubmittedPayload{
		SearchID: storeSearchID, SearchVersion: event.SearchVersionFTS5V1, Query: "first",
	})
	if _, err := store.AppendBatch(ctx, []event.Record{first}, storeReceivedAt(0)); err != nil {
		t.Fatalf("append first: %v", err)
	}
	second := makeEventRecord(t, storeEventID, storeUserIDTwo, event.KindSearchSubmitted, event.SearchSubmittedPayload{
		SearchID: storeSearchIDTwo, SearchVersion: event.SearchVersionFTS5V1, Query: "second",
	})
	result, err := store.AppendBatch(ctx, []event.Record{second}, storeReceivedAt(time.Millisecond))
	if err != nil {
		t.Fatalf("append replay: %v", err)
	}
	if result.AcceptedCount != 0 || result.DuplicateCount != 1 {
		t.Fatalf("replay result = %+v", result)
	}
	var query, userID, searchID string
	if err := db.QueryRow(`SELECT json_extract(payload_json, '$.query'), user_id, search_id FROM events WHERE id = ?`, storeEventID).Scan(&query, &userID, &searchID); err != nil {
		t.Fatalf("query replayed event: %v", err)
	}
	if query != "first" || userID != storeUserID || searchID != storeSearchID {
		t.Fatalf("first row changed: query=%q user=%q search=%q", query, userID, searchID)
	}
}

func TestEventStoreAppendBatchRollsBackOnForeignKeyFailure(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	store := NewEventStore(db)
	valid := makeEventRecord(t, storeEventID, storeUserID, event.KindNotePublished, event.NotePublishedPayload{NoteID: storeEventID, CategorySlug: "food"})
	missingUser := makeEventRecord(t, storeEventIDTwo, "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d18", event.KindNotePublished, event.NotePublishedPayload{NoteID: storeEventID, CategorySlug: "food"})
	if _, err := store.AppendBatch(ctx, []event.Record{valid, missingUser}, storeReceivedAt(0)); err == nil {
		t.Fatal("append with missing user error = nil")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("event count after rollback = %d, want 0", count)
	}
}

func TestEventStoreDeleteUserCascadesEvents(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	store := NewEventStore(db)
	record := makeEventRecord(t, storeEventID, storeUserID, event.KindNotePublished, event.NotePublishedPayload{NoteID: storeEventID, CategorySlug: "food"})
	if _, err := store.AppendBatch(ctx, []event.Record{record}, storeReceivedAt(0)); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = ?`, storeUserID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("event count after user delete = %d, want 0", count)
	}
}
func TestEventStoreStreamsExportRowsInInsertionOrder(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	store := NewEventStore(db)

	first := makeEventRecord(t, storeEventID, storeUserID, event.KindNotePublished, event.NotePublishedPayload{
		NoteID: storeEventID, CategorySlug: "food",
	})
	first.InstallationID = nil
	first.AppVersion = nil
	second := makeEventRecord(t, storeEventIDTwo, storeUserID, event.KindSearchSubmitted, event.SearchSubmittedPayload{
		SearchID: storeSearchID, SearchVersion: event.SearchVersionFTS5V1, Query: "evento",
	})
	receivedAt := storeReceivedAt(time.Millisecond)
	if _, err := store.AppendBatch(ctx, []event.Record{first, second}, receivedAt); err != nil {
		t.Fatalf("append events: %v", err)
	}

	rows := make([]EventExportRow, 0, 2)
	if err := store.StreamExportRows(ctx, func(row EventExportRow) error {
		rows = append(rows, row)
		return nil
	}); err != nil {
		t.Fatalf("stream export rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("export rows = %d, want 2", len(rows))
	}
	if rows[0].EventPageKey != 1 || rows[0].ID != storeEventID ||
		rows[1].EventPageKey != 2 || rows[1].ID != storeEventIDTwo {
		t.Fatalf("export order = %+v", rows)
	}
	if rows[0].InstallationID != nil || rows[0].AppVersion != nil {
		t.Fatalf("nullable fields = installation %v app %v, want nil", rows[0].InstallationID, rows[0].AppVersion)
	}
	if rows[1].InstallationID == nil || rows[1].AppVersion == nil {
		t.Fatal("non-null event fields were lost")
	}
	if rows[0].ReceivedAt != receivedAt.UnixMilli() || rows[1].ReceivedAt != receivedAt.UnixMilli() {
		t.Fatalf("received_at = %d/%d, want %d", rows[0].ReceivedAt, rows[1].ReceivedAt, receivedAt.UnixMilli())
	}
	if len(rows[0].Payload) == 0 || rows[0].Payload[0] != '{' {
		t.Fatalf("payload = %q, want JSON object", rows[0].Payload)
	}
	var payload map[string]any
	if err := json.Unmarshal(rows[1].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["query"] != "evento" {
		t.Fatalf("query payload = %#v", payload["query"])
	}
}

func TestEventStoreAppendBatchPersistsCommentParentAsNullOrString(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertEventTestUser(t, db, storeUserID)
	store := NewEventStore(db)

	topLevel := makeEventRecord(t, storeEventIDComment, storeUserID, event.KindCommentCreated, event.CommentCreatedPayload{
		NoteID:    storeEventID,
		CommentID: storeCommentID,
	})
	reply := makeEventRecord(t, storeEventIDCommentReply, storeUserID, event.KindCommentCreated, event.CommentCreatedPayload{
		NoteID:          storeEventID,
		CommentID:       storeReplyCommentID,
		ParentCommentID: new(comment.CommentID(storeParentCommentID)),
	})
	if _, err := store.AppendBatch(ctx, []event.Record{topLevel, reply}, storeReceivedAt(0)); err != nil {
		t.Fatalf("append comment events: %v", err)
	}

	var topLevelPayload string
	if err := db.QueryRow(`SELECT payload_json FROM events WHERE id = ?`, storeEventIDComment).Scan(&topLevelPayload); err != nil {
		t.Fatalf("query top-level comment payload: %v", err)
	}
	if !strings.Contains(topLevelPayload, `"parent_comment_id":null`) {
		t.Fatalf("top-level comment payload = %q, want parent_comment_id:null", topLevelPayload)
	}

	var replyParent string
	if err := db.QueryRow(`SELECT json_extract(payload_json, '$.parent_comment_id') FROM events WHERE id = ?`, storeEventIDCommentReply).Scan(&replyParent); err != nil {
		t.Fatalf("query reply comment payload: %v", err)
	}
	if replyParent != storeParentCommentID {
		t.Fatalf("reply parent_comment_id = %q, want %q", replyParent, storeParentCommentID)
	}
}

func makeEventRecord(t *testing.T, id, userID string, kind event.Kind, payload event.Payload) event.Record {
	t.Helper()
	record, problems := event.NormalizeAndValidate(event.Input{
		ID:             id,
		Kind:           kind,
		OccurredAt:     time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC).UnixMilli(),
		UserID:         user.UserID(userID),
		InstallationID: new(storeInstallID),
		Platform:       event.PlatformWeb,
		AppVersion:     new("0.0.1"),
		SchemaVersion:  event.SchemaVersion1,
		Payload:        payload,
	})
	if len(problems) > 0 {
		t.Fatalf("make event record: %+v", problems)
	}
	return record
}

func insertEventTestUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`, id, eventOccurredAt().UnixMilli(), eventOccurredAt().UnixMilli()); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func eventOccurredAt() time.Time {
	return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
}

func storeReceivedAt(offset time.Duration) time.Time {
	return time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC).Add(offset)
}

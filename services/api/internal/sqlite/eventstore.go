package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/event"
)

type EventStore struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{db: db}
}

const insertEventSQL = `
	INSERT INTO events (
		id,
		kind,
		occurred_at,
		received_at,
		user_id,
		installation_id,
		app_platform,
		app_version,
		schema_version,
		search_id,
		payload_json
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO NOTHING
`

func (store *EventStore) AppendBatch(ctx context.Context, records []event.Record, receivedAt time.Time) (event.AppendBatchResult, error) {
	if len(records) == 0 {
		return event.AppendBatchResult{}, nil
	}

	receivedAt = normalizeTime(receivedAt)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return event.AppendBatchResult{}, fmt.Errorf("begin append events: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result := event.AppendBatchResult{}
	for index, record := range records {
		payloadJSON, err := json.Marshal(record.Payload)
		if err != nil {
			return event.AppendBatchResult{}, fmt.Errorf("marshal event payload %d: %w", index, err)
		}
		searchID := searchIDForPayload(record.Payload)
		insertResult, err := tx.ExecContext(
			ctx,
			insertEventSQL,
			record.ID,
			string(record.Kind),
			unixMillis(record.OccurredAt),
			unixMillis(receivedAt),
			string(record.UserID),
			record.InstallationID,
			string(record.Platform),
			record.AppVersion,
			record.SchemaVersion,
			searchID,
			string(payloadJSON),
		)
		if err != nil {
			return event.AppendBatchResult{}, fmt.Errorf("insert event %d: %w", index, err)
		}
		affected, err := insertResult.RowsAffected()
		if err != nil {
			return event.AppendBatchResult{}, fmt.Errorf("read event %d result: %w", index, err)
		}
		switch affected {
		case 1:
			result.AcceptedCount++
		case 0:
			result.DuplicateCount++
		default:
			return event.AppendBatchResult{}, fmt.Errorf("insert event %d affected %d rows", index, affected)
		}
	}

	if err := tx.Commit(); err != nil {
		return event.AppendBatchResult{}, fmt.Errorf("commit events: %w", err)
	}
	return result, nil
}

func searchIDForPayload(payload event.Payload) any {
	switch value := payload.(type) {
	case event.SearchSubmittedPayload:
		return value.SearchID
	case event.SearchResultsImpressionPayload:
		return value.SearchID
	case event.SearchResultOpenedPayload:
		return value.SearchID
	case event.SearchNoResultsPayload:
		return value.SearchID
	case event.SearchReformulatedPayload:
		return value.SearchID
	case event.NoteMarkedUsefulPayload:
		if context, ok := value.Context.(event.SearchUsefulContext); ok {
			return context.SearchID
		}
	case event.NoteUnmarkedUsefulPayload:
		if context, ok := value.Context.(event.SearchUsefulContext); ok {
			return context.SearchID
		}
	}
	return nil
}

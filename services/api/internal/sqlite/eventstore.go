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

type EventExportRow struct {
	EventPageKey   int64
	ID             string
	Kind           string
	OccurredAt     int64
	ReceivedAt     int64
	UserID         string
	InstallationID *string
	Platform       string
	AppVersion     *string
	SchemaVersion  int
	Payload        json.RawMessage
}

const streamEventsSQL = `
	SELECT
		event_page_key,
		id,
		kind,
		occurred_at,
		received_at,
		user_id,
		installation_id,
		app_platform,
		app_version,
		schema_version,
		payload_json
	FROM events
	ORDER BY event_page_key ASC
`

// StreamExportRows passes each scanned row by value so callers cannot mutate
// store-owned scan state or retain a pointer into the iteration.
func (store *EventStore) StreamExportRows(
	ctx context.Context,
	visit func(EventExportRow) error,
) error {
	if visit == nil {
		return fmt.Errorf("stream event export rows: nil visitor")
	}
	rows, err := store.db.QueryContext(ctx, streamEventsSQL)
	if err != nil {
		return fmt.Errorf("query event export rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			row            EventExportRow
			installationID sql.NullString
			appVersion     sql.NullString
			payload        []byte
		)
		if err := rows.Scan(
			&row.EventPageKey,
			&row.ID,
			&row.Kind,
			&row.OccurredAt,
			&row.ReceivedAt,
			&row.UserID,
			&installationID,
			&row.Platform,
			&appVersion,
			&row.SchemaVersion,
			&payload,
		); err != nil {
			return fmt.Errorf("scan event export row: %w", err)
		}
		if installationID.Valid {
			value := installationID.String
			row.InstallationID = &value
		}
		if appVersion.Valid {
			value := appVersion.String
			row.AppVersion = &value
		}
		if !json.Valid(payload) {
			return fmt.Errorf("event export row %d has invalid payload JSON", row.EventPageKey)
		}
		row.Payload = append(json.RawMessage(nil), payload...)
		if err := visit(row); err != nil {
			return fmt.Errorf("visit event export row %d: %w", row.EventPageKey, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read event export rows: %w", err)
	}
	return nil
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

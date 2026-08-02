package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/embedding"
	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/eventexport"
	"github.com/tprei/sdds/services/api/internal/s3store"
	"github.com/tprei/sdds/services/api/internal/sqlite"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	exportUserID     = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d21"
	exportEventID    = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d22"
	exportEventIDTwo = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d23"
	exportSearchID   = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d24"
)

type decodedEventExportRow struct {
	EventPageKey   int64           `json:"event_page_key"`
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	OccurredAt     int64           `json:"occurred_at"`
	ReceivedAt     int64           `json:"received_at"`
	UserID         string          `json:"user_id"`
	InstallationID *string         `json:"installation_id"`
	Platform       string          `json:"platform"`
	AppVersion     *string         `json:"app_version"`
	SchemaVersion  int             `json:"schema_version"`
	Payload        json.RawMessage `json:"payload"`
}

func TestRunExportEventsStreamsRowsAsNDJSON(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedExportEventsDatabase(t, databasePath)
	output := captureEventOutput(t)

	if err := eventexport.Run(ctx, databasePath, eventOutputStream); err != nil {
		t.Fatalf("run export events: %v", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	rows := make([]decodedEventExportRow, 0, 2)
	for {
		var row decodedEventExportRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode export row: %v", err)
		}
		rows = append(rows, row)
	}
	if len(rows) != 2 {
		t.Fatalf("export rows = %d, want 2", len(rows))
	}
	if rows[0].EventPageKey != 1 || rows[0].ID != exportEventID ||
		rows[1].EventPageKey != 2 || rows[1].ID != exportEventIDTwo {
		t.Fatalf("export order = %+v", rows)
	}
	if rows[0].UserID != exportUserID || rows[0].ReceivedAt != exportTestTime().UnixMilli() {
		t.Fatalf("export identity/timestamp = %+v", rows[0])
	}
	if rows[0].InstallationID != nil || rows[0].AppVersion != nil {
		t.Fatalf("nullable envelope fields = installation %v app %v, want nil", rows[0].InstallationID, rows[0].AppVersion)
	}
	if len(rows[0].Payload) == 0 || rows[0].Payload[0] != '{' {
		t.Fatalf("payload = %q, want embedded object", rows[0].Payload)
	}
	var payload map[string]any
	if err := json.Unmarshal(rows[1].Payload, &payload); err != nil {
		t.Fatalf("decode embedded payload: %v", err)
	}
	if payload["search_id"] != exportSearchID {
		t.Fatalf("search payload = %#v", payload)
	}
}

func TestRunWithArgsExportEvents(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedExportEventsDatabase(t, databasePath)
	output := captureEventOutput(t)

	if err := runWithArgs(context.Background(), config{databasePath: databasePath}, s3store.Config{}, embedding.Config{}, []string{commandExportEvents}); err != nil {
		t.Fatalf("run export-events command: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("export-events output is empty")
	}
}

func captureEventOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	original := eventOutputStream
	output := &bytes.Buffer{}
	eventOutputStream = output
	t.Cleanup(func() { eventOutputStream = original })
	return output
}

func seedExportEventsDatabase(t *testing.T, databasePath string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`, exportUserID, exportTestTime().UnixMilli(), exportTestTime().UnixMilli()); err != nil {
		t.Fatalf("insert export user: %v", err)
	}

	first := normalizeExportEvent(t, event.Input{
		ID: exportEventID, Kind: event.KindNotePublished, OccurredAt: exportTestTime().Add(-100 * time.Millisecond).UnixMilli(),
		UserID: user.UserID(exportUserID), Platform: event.PlatformWeb,
		SchemaVersion: event.SchemaVersion1,
		Payload:       event.NotePublishedPayload{NoteID: exportEventIDTwo, CategorySlug: "food"},
	})
	second := normalizeExportEvent(t, event.Input{
		ID: exportEventIDTwo, Kind: event.KindSearchSubmitted, OccurredAt: exportTestTime().Add(-99 * time.Millisecond).UnixMilli(),
		UserID: user.UserID(exportUserID), InstallationID: stringPointer("018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d25"),
		Platform: event.PlatformWeb, AppVersion: stringPointer("0.0.1"), SchemaVersion: event.SchemaVersion1,
		Payload: event.SearchSubmittedPayload{SearchID: exportSearchID, SearchVersion: event.SearchVersionFTS5V1, Query: "evento"},
	})
	if _, err := sqlite.NewEventStore(db).AppendBatch(ctx, []event.Record{first, second}, exportTestTime()); err != nil {
		t.Fatalf("append export events: %v", err)
	}
}

func exportTestTime() time.Time {
	return time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)
}

func normalizeExportEvent(t *testing.T, input event.Input) event.Record {
	t.Helper()
	record, problems := event.NormalizeAndValidate(input)
	if len(problems) > 0 {
		t.Fatalf("normalize export event: %+v", problems)
	}
	return record
}

func stringPointer(value string) *string {
	return &value
}

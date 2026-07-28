package eventexport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/tprei/sdds/services/api/internal/sqlite"
)

type outputRow struct {
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

// Run opens the database read-only and streams events in event_page_key order.
func Run(ctx context.Context, databasePath string, output io.Writer) error {
	db, err := sqlite.OpenReadOnly(databasePath)
	if err != nil {
		return fmt.Errorf("open database read-only: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("close read-only database", "error", closeErr)
		}
	}()

	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := sqlite.NewEventStore(db).StreamExportRows(ctx, func(row sqlite.EventExportRow) error {
		return encoder.Encode(outputRow{
			EventPageKey:   row.EventPageKey,
			ID:             row.ID,
			Kind:           row.Kind,
			OccurredAt:     row.OccurredAt,
			ReceivedAt:     row.ReceivedAt,
			UserID:         row.UserID,
			InstallationID: row.InstallationID,
			Platform:       row.Platform,
			AppVersion:     row.AppVersion,
			SchemaVersion:  row.SchemaVersion,
			Payload:        row.Payload,
		})
	}); err != nil {
		return fmt.Errorf("export events: %w", err)
	}
	return nil
}

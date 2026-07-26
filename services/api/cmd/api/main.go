package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/s3store"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

const (
	commandMigrate          = "migrate"
	commandInspectReports   = "inspect-reports"
	commandExportEvents     = "export-events"
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 15 * time.Second
	startupReadinessTimeout = 5 * time.Second
)

type databaseReadiness interface {
	PingContext(context.Context) error
}

type mediaReadiness interface {
	VerifyReadiness(context.Context) error
}

type readyObjectStore interface {
	media.ObjectStore
	mediaReadiness
}

type runtimeReadiness struct {
	database databaseReadiness
	media    mediaReadiness
}

func (readiness runtimeReadiness) Check(ctx context.Context) error {
	if readiness.database == nil {
		return errors.New("database readiness is unavailable")
	}
	if err := readiness.database.PingContext(ctx); err != nil {
		return fmt.Errorf("database readiness: %w", err)
	}
	if readiness.media == nil {
		return errors.New("media readiness is unavailable")
	}
	if err := readiness.media.VerifyReadiness(ctx); err != nil {
		return fmt.Errorf("media readiness: %w", err)
	}
	return nil
}

var newMediaStore = func(ctx context.Context, config s3store.Config) (readyObjectStore, error) {
	store, err := s3store.New(ctx, config)
	return store, err
}

var loadS3Config = s3store.LoadConfigFromEnv

var listenAndServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

var closeDatabase = func(database *sql.DB) error {
	return database.Close()
}

var reportOutputStream io.Writer = os.Stdout
var eventOutputStream io.Writer = os.Stdout

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && (len(args) != 1 || (args[0] != commandMigrate && args[0] != commandInspectReports && args[0] != commandExportEvents)) {
		return runWithArgs(context.Background(), config{}, s3store.Config{}, args)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		return runWithArgs(context.Background(), cfg, s3store.Config{}, args)
	}
	s3Config, err := loadS3Config()
	if err != nil {
		return err
	}
	return runWithArgs(context.Background(), cfg, s3Config, args)
}

func runWithArgs(ctx context.Context, config config, s3Config s3store.Config, args []string) error {
	switch {
	case len(args) == 0:
		return runServer(ctx, config, s3Config)
	case len(args) == 1 && args[0] == commandMigrate:
		return runMigrations(ctx, config)
	case len(args) == 1 && args[0] == commandInspectReports:
		return runInspectReports(ctx, config)
	case len(args) == 1 && args[0] == commandExportEvents:
		return runExportEvents(ctx, config)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMigrations(ctx context.Context, config config) (err error) {
	db, err := openMigratedDatabase(ctx, config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	return nil
}

func runServer(ctx context.Context, config config, s3Config s3store.Config) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := openMigratedDatabase(ctx, config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeDatabase(db); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	store, err := newMediaStore(ctx, s3Config)
	if err != nil {
		return fmt.Errorf("create media store: %w", err)
	}
	readinessCtx, cancel := context.WithTimeout(ctx, startupReadinessTimeout)
	defer cancel()
	if err := store.VerifyReadiness(readinessCtx); err != nil {
		return fmt.Errorf("verify media readiness: %w", err)
	}
	noteStore := sqlite.NewNoteStore(db)
	commentStore := sqlite.NewCommentStore(db)
	catalogStore := sqlite.NewCatalogStore(db)
	userStore := sqlite.NewUserStore(db)
	uploadStore := sqlite.NewImageUploadStore(db)
	uploadService, err := media.NewUploadService(uploadStore, store, media.UploadConfig{})
	if err != nil {
		return fmt.Errorf("create upload service: %w", err)
	}
	imageReader := media.NewImageReader(noteStore, store)
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, startupReadinessTimeout)
	if err := uploadService.CleanupExpired(cleanupCtx, time.Now()); err != nil {
		cleanupCancel()
		return fmt.Errorf("cleanup expired uploads: %w", err)
	}
	cleanupCancel()
	readiness := runtimeReadiness{database: db, media: store}
	server := newServer(config, httpapi.NewRouter(
		httpapi.NotesDependencies{Stores: noteStore, Catalog: catalogStore},
		httpapi.CommentDependencies{Store: commentStore},
		httpapi.ReportDependencies{Store: sqlite.NewReportStore(db), CommentTargets: commentStore},
		httpapi.EventDependencies{Store: sqlite.NewEventStore(db), Limits: httpapi.DefaultEventLimits()},
		httpapi.AuthDependencies{Users: userStore, Limits: config.authLimits},
		httpapi.MediaDependencies{ImageUploads: uploadService, AttachedImages: imageReader},
		httpapi.SystemDependencies{Readiness: readiness},
	))

	slog.Info("api listening", "addr", config.httpAddr)
	if err := listenAndServe(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func openMigratedDatabase(ctx context.Context, config config) (*sql.DB, error) {
	db, err := sqlite.Open(config.databasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("apply migrations: %w; close database: %v", err, closeErr)
		}
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return db, nil
}

func newServer(config config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              config.httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
	}
}

// reportOutputRow is the JSON Lines record emitted by inspect-reports. Field
// declaration order is the emitted JSON key order.
type reportOutputRow struct {
	ReportPageKey  int64   `json:"report_page_key"`
	ID             string  `json:"id"`
	CreatedAt      int64   `json:"created_at"`
	ReporterUserID string  `json:"reporter_user_id"`
	TargetType     string  `json:"target_type"`
	TargetID       string  `json:"target_id"`
	Reason         string  `json:"reason"`
	Details        *string `json:"details"`
	TargetSummary  *string `json:"target_summary"`
	TargetMissing  int     `json:"target_missing"`
}

// eventOutputRow is the JSON Lines record emitted by export-events. Field
// declaration order is the emitted JSON key order.
type eventOutputRow struct {
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

// runInspectReports opens the database read-only, loads the operator
// inspection rows from the report store, and prints one compact JSON object
// per row ordered by report_page_key ascending. It never runs migrations and
// writes nothing to stdout on failure.
func runInspectReports(ctx context.Context, config config) error {
	db, err := sqlite.OpenReadOnly(config.databasePath)
	if err != nil {
		return fmt.Errorf("open database read-only: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("close read-only database", "error", closeErr)
		}
	}()

	rows, err := sqlite.NewReportStore(db).ListInspectionRows(ctx)
	if err != nil {
		return fmt.Errorf("load report inspection rows: %w", err)
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, row := range rows {
		if err := encoder.Encode(reportOutputRow{
			ReportPageKey:  row.ReportPageKey,
			ID:             row.ID,
			CreatedAt:      row.CreatedAt,
			ReporterUserID: row.ReporterUserID,
			TargetType:     string(row.TargetType),
			TargetID:       row.TargetID,
			Reason:         string(row.Reason),
			Details:        row.Details,
			TargetSummary:  row.TargetSummary,
			TargetMissing:  boolToInt(row.TargetMissing),
		}); err != nil {
			return fmt.Errorf("encode report row: %w", err)
		}
	}
	if _, err := output.WriteTo(reportOutputStream); err != nil {
		return fmt.Errorf("write report rows: %w", err)
	}
	return nil
}

// runExportEvents opens the database read-only and streams one compact JSON
// object per event ordered by event_page_key ascending. It never runs
// migrations or loads media configuration.
func runExportEvents(ctx context.Context, config config) error {
	db, err := sqlite.OpenReadOnly(config.databasePath)
	if err != nil {
		return fmt.Errorf("open database read-only: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("close read-only database", "error", closeErr)
		}
	}()

	encoder := json.NewEncoder(eventOutputStream)
	encoder.SetEscapeHTML(false)
	if err := sqlite.NewEventStore(db).StreamExportRows(
		ctx,
		func(row sqlite.EventExportRow) error {
			return encoder.Encode(eventOutputRow{
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
		},
	); err != nil {
		return fmt.Errorf("export events: %w", err)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

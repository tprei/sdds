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

	"github.com/tprei/sdds/services/api/internal/embedding"
	"github.com/tprei/sdds/services/api/internal/eventexport"
	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/s3store"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

const (
	commandMigrate           = "migrate"
	commandInspectReports    = "inspect-reports"
	commandExportEvents      = "export-events"
	commandReindexEmbeddings = "reindex-embeddings"
	serverReadHeaderTimeout  = 5 * time.Second
	serverReadTimeout        = 15 * time.Second
	startupReadinessTimeout  = 5 * time.Second
)

type databaseReadiness interface {
	PingContext(context.Context) error
}

type mediaReadiness interface {
	VerifyReadiness(context.Context) error
}

type embeddingReadiness interface {
	VerifyReadiness(context.Context) error
}

type readyObjectStore interface {
	media.ObjectStore
	mediaReadiness
}

// embeddingStore is what runServer needs from the embedding sidecar: startup
// readiness plus the embed operations note.Publisher uses to embed a note's
// passage before it is ever persisted.
type embeddingStore interface {
	note.Embedder
	embeddingReadiness
}

type runtimeReadiness struct {
	database  databaseReadiness
	media     mediaReadiness
	embedding embeddingReadiness
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
	if readiness.embedding == nil {
		return errors.New("embedding readiness is unavailable")
	}
	if err := readiness.embedding.VerifyReadiness(ctx); err != nil {
		return fmt.Errorf("embedding readiness: %w", err)
	}
	return nil
}

var newMediaStore = func(ctx context.Context, config s3store.Config) (readyObjectStore, error) {
	store, err := s3store.New(ctx, config)
	return store, err
}

var loadS3Config = s3store.LoadConfigFromEnv

var newEmbeddingClient = func(config embedding.Config) (embeddingStore, error) {
	return embedding.New(config)
}

var loadEmbeddingConfig = embedding.LoadConfigFromEnv

var listenAndServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

var closeDatabase = func(database *sql.DB) error {
	return database.Close()
}

var reportOutputStream io.Writer = os.Stdout
var eventOutputStream io.Writer = os.Stdout
var reindexOutputStream io.Writer = os.Stdout

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

// run dispatches by argument count and command name. Commands that never
// touch S3 or the embedding sidecar (migrate, inspect-reports, export-events)
// load only the app config. Server mode and reindex-embeddings both need the
// embedding client, so both load the full config set.
func run() error {
	args := os.Args[1:]
	bareConfigCommand := len(args) == 1 && (args[0] == commandMigrate || args[0] == commandInspectReports || args[0] == commandExportEvents)
	fullConfigCommand := len(args) == 0 || (len(args) == 1 && args[0] == commandReindexEmbeddings)
	if !bareConfigCommand && !fullConfigCommand {
		return runWithArgs(context.Background(), config{}, s3store.Config{}, embedding.Config{}, args)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if bareConfigCommand {
		return runWithArgs(context.Background(), cfg, s3store.Config{}, embedding.Config{}, args)
	}
	s3Config, err := loadS3Config()
	if err != nil {
		return err
	}
	embeddingConfig, err := loadEmbeddingConfig()
	if err != nil {
		return err
	}
	return runWithArgs(context.Background(), cfg, s3Config, embeddingConfig, args)
}

func runWithArgs(ctx context.Context, config config, s3Config s3store.Config, embeddingConfig embedding.Config, args []string) error {
	switch {
	case len(args) == 0:
		return runServer(ctx, config, s3Config, embeddingConfig)
	case len(args) == 1 && args[0] == commandMigrate:
		return runMigrations(ctx, config)
	case len(args) == 1 && args[0] == commandInspectReports:
		return runInspectReports(ctx, config)
	case len(args) == 1 && args[0] == commandExportEvents:
		return eventexport.Run(ctx, config.databasePath, eventOutputStream)
	case len(args) == 1 && args[0] == commandReindexEmbeddings:
		return runReindexEmbeddings(ctx, config, embeddingConfig)
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

// reindexOutputRow is the compact JSON summary printed after a reindex run.
// Field declaration order is the emitted JSON key order.
type reindexOutputRow struct {
	Scanned  int `json:"scanned"`
	Embedded int `json:"embedded"`
	Skipped  int `json:"skipped"`
}

// runReindexEmbeddings backfills or repairs note embeddings idempotently: a
// note whose stored model id, revision, and source fingerprint already match
// the current values is skipped, so running this twice in a row embeds
// nothing the second time. It writes nothing to stdout on failure.
func runReindexEmbeddings(ctx context.Context, config config, embeddingConfig embedding.Config) (err error) {
	db, err := openMigratedDatabase(ctx, config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	embeddingClient, err := newEmbeddingClient(embeddingConfig)
	if err != nil {
		return fmt.Errorf("create embedding client: %w", err)
	}
	readinessCtx, cancel := context.WithTimeout(ctx, startupReadinessTimeout)
	defer cancel()
	if err := embeddingClient.VerifyReadiness(readinessCtx); err != nil {
		return fmt.Errorf("verify embedding readiness: %w", err)
	}

	noteStore := sqlite.NewNoteStore(db)
	result, err := note.ReindexEmbeddings(ctx, noteStore, embeddingClient, time.Now)
	if err != nil {
		return fmt.Errorf("reindex embeddings: %w", err)
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(reindexOutputRow{Scanned: result.Scanned, Embedded: result.Embedded, Skipped: result.Skipped}); err != nil {
		return fmt.Errorf("encode reindex summary: %w", err)
	}
	if _, err := output.WriteTo(reindexOutputStream); err != nil {
		return fmt.Errorf("write reindex summary: %w", err)
	}
	return nil
}

func runServer(ctx context.Context, config config, s3Config s3store.Config, embeddingConfig embedding.Config) (err error) {
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
	embeddingClient, err := newEmbeddingClient(embeddingConfig)
	if err != nil {
		return fmt.Errorf("create embedding client: %w", err)
	}
	embeddingReadinessCtx, embeddingCancel := context.WithTimeout(ctx, startupReadinessTimeout)
	defer embeddingCancel()
	if err := embeddingClient.VerifyReadiness(embeddingReadinessCtx); err != nil {
		return fmt.Errorf("verify embedding readiness: %w", err)
	}
	noteStore := sqlite.NewNoteStore(db)
	publisher := note.NewPublisher(noteStore, embeddingClient)
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
	readiness := runtimeReadiness{database: db, media: store, embedding: embeddingClient}
	server := newServer(config, httpapi.NewRouter(
		httpapi.NotesDependencies{Stores: noteStore, Publisher: publisher, Catalog: catalogStore},
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

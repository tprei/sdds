package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && (len(args) != 1 || args[0] != commandMigrate) {
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

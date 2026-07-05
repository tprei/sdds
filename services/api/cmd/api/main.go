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
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

const (
	commandMigrate          = "migrate"
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	return runWithArgs(context.Background(), loadConfig(), os.Args[1:])
}

func runWithArgs(ctx context.Context, config config, args []string) error {
	switch {
	case len(args) == 0:
		return runServer(ctx, config)
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

func runServer(ctx context.Context, config config) (err error) {
	db, err := openMigratedDatabase(ctx, config)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	noteStore := sqlite.NewNoteStore(db)
	server := newServer(config, httpapi.NewRouter(noteStore))

	slog.Info("api listening", "addr", config.httpAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

package main

import (
	"context"
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
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() (err error) {
	ctx := context.Background()
	config := loadConfig()

	db, err := sqlite.Open(config.databasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	noteStore := sqlite.NewNoteStore(db)
	server := newServer(config, httpapi.NewRouter(noteStore))

	slog.Info("api listening", "addr", config.httpAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func newServer(config config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              config.httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
	}
}

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

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	config := loadConfig()

	db, err := sqlite.Open(config.databasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	noteStore := sqlite.NewNoteStore(db)
	server := &http.Server{
		Addr:              config.httpAddr,
		Handler:           httpapi.NewRouter(noteStore),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("api listening", "addr", config.httpAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

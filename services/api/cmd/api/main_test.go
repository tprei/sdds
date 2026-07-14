package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

func TestNewServerSetsReadTimeouts(t *testing.T) {
	server := newServer(config{httpAddr: ":9090"}, http.NotFoundHandler())

	got := serverSettings{
		Addr:              server.Addr,
		ReadHeaderTimeout: server.ReadHeaderTimeout,
		ReadTimeout:       server.ReadTimeout,
	}
	want := serverSettings{
		Addr:              ":9090",
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("server settings mismatch (-want +got):\n%s", diff)
	}
	if server.Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestRunWithArgsAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "sdds.db")

	if err := runWithArgs(ctx, config{databasePath: databasePath}, []string{"migrate"}); err != nil {
		t.Fatalf("run migrate command: %v", err)
	}

	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close migrated database: %v", err)
		}
	}()

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'notes'`).Scan(&count); err != nil {
		t.Fatalf("query migrated notes table: %v", err)
	}
	if count != 1 {
		t.Fatalf("notes table count = %d, want 1", count)
	}
}

func TestRunMigrateDoesNotLoadMediaConfig(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SDDS_DATABASE_PATH", filepath.Join(t.TempDir(), "sdds.db"))
	originalArgs := os.Args
	os.Args = []string{"api", commandMigrate}
	t.Cleanup(func() { os.Args = originalArgs })
	if err := run(); err != nil {
		t.Fatalf("run migrate command without media secrets: %v", err)
	}
}

func TestRunWithArgsRejectsUnknownCommand(t *testing.T) {
	err := runWithArgs(context.Background(), config{}, []string{"unknown"})
	if err == nil {
		t.Fatal("unknown command error is nil")
	}
	if got, want := err.Error(), `unknown command "unknown"`; got != want {
		t.Fatalf("unknown command error = %q, want %q", got, want)
	}
}

type serverSettings struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
}

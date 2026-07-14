package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
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

func TestRunServerRequiresMediaReadiness(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing sentinel", err: media.ErrObjectNotFound},
		{name: "unavailable sentinel", err: media.ErrObjectUnavailable},
		{name: "mismatched sentinel", err: media.ErrObjectIntegrity},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restoreMediaStoreFactory(t)
			newMediaStore = func(context.Context, media.Config) (media.ReadinessChecker, error) {
				return fakeMediaReadiness{verify: func(context.Context) error { return test.err }}, nil
			}
			listened := false
			restoreListen := listenAndServe
			listenAndServe = func(*http.Server) error {
				listened = true
				return http.ErrServerClosed
			}
			t.Cleanup(func() { listenAndServe = restoreListen })

			err := runServer(context.Background(), testServerConfig(t))
			if !errors.Is(err, test.err) {
				t.Fatalf("run server error = %v, want %v", err, test.err)
			}
			if listened {
				t.Fatal("server listened despite failed media readiness")
			}
		})
	}
}

func TestRunServerListensAfterMediaReadiness(t *testing.T) {
	restoreMediaStoreFactory(t)
	verified := false
	newMediaStore = func(context.Context, media.Config) (media.ReadinessChecker, error) {
		return fakeMediaReadiness{verify: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("startup readiness context has no deadline")
			}
			verified = true
			return nil
		}}, nil
	}

	listened := false
	restoreListen := listenAndServe
	listenAndServe = func(*http.Server) error {
		if !verified {
			t.Fatal("server listened before media readiness")
		}
		listened = true
		return http.ErrServerClosed
	}
	t.Cleanup(func() { listenAndServe = restoreListen })

	if err := runServer(context.Background(), testServerConfig(t)); err != nil {
		t.Fatalf("run server: %v", err)
	}
	if !listened {
		t.Fatal("server was not asked to listen")
	}
}

type fakeMediaReadiness struct {
	verify func(context.Context) error
}

func (fake fakeMediaReadiness) VerifyReadiness(ctx context.Context) error {
	return fake.verify(ctx)
}

func restoreMediaStoreFactory(t *testing.T) {
	t.Helper()
	original := newMediaStore
	t.Cleanup(func() { newMediaStore = original })
}

func testServerConfig(t *testing.T) config {
	t.Helper()
	return config{
		authLimits:   httpapi.DefaultAuthLimits(),
		databasePath: filepath.Join(t.TempDir(), "sdds.db"),
		httpAddr:     "127.0.0.1:0",
	}
}

type serverSettings struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
}

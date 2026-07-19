package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/s3store"
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

	if err := runWithArgs(ctx, config{databasePath: databasePath}, s3store.Config{}, []string{"migrate"}); err != nil {
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

func TestRunMigrateDoesNotLoadS3Config(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SDDS_DATABASE_PATH", filepath.Join(t.TempDir(), "sdds.db"))
	originalArgs := os.Args
	os.Args = []string{"api", commandMigrate}
	t.Cleanup(func() { os.Args = originalArgs })
	restoreS3ConfigLoader(t)
	loaded := false
	loadS3Config = func() (s3store.Config, error) {
		loaded = true
		return s3store.Config{}, errors.New("S3 config loader called")
	}
	if err := run(); err != nil {
		t.Fatalf("run migrate command without media secrets: %v", err)
	}
	if loaded {
		t.Fatal("migrate loaded S3 config")
	}
}

func TestRunLoadsS3ConfigForServer(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SDDS_DATABASE_PATH", filepath.Join(t.TempDir(), "sdds.db"))
	originalArgs := os.Args
	os.Args = []string{"api"}
	t.Cleanup(func() { os.Args = originalArgs })
	restoreS3ConfigLoader(t)
	loaded := false
	loadS3Config = func() (s3store.Config, error) {
		loaded = true
		return s3store.Config{}, nil
	}
	restoreMediaStoreFactory(t)
	newMediaStore = func(context.Context, s3store.Config) (readyObjectStore, error) {
		return fakeMediaReadiness{verify: func(context.Context) error { return nil }}, nil
	}
	restoreListen := listenAndServe
	listenAndServe = func(*http.Server) error { return http.ErrServerClosed }
	t.Cleanup(func() { listenAndServe = restoreListen })

	if err := run(); err != nil {
		t.Fatalf("run server: %v", err)
	}
	if !loaded {
		t.Fatal("server did not load S3 config")
	}
}

func TestRunWithArgsRejectsUnknownCommand(t *testing.T) {
	err := runWithArgs(context.Background(), config{}, s3store.Config{}, []string{"unknown"})
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
			newMediaStore = func(context.Context, s3store.Config) (readyObjectStore, error) {
				return fakeMediaReadiness{verify: func(context.Context) error { return test.err }}, nil
			}
			listened := false
			restoreListen := listenAndServe
			listenAndServe = func(*http.Server) error {
				listened = true
				return http.ErrServerClosed
			}
			t.Cleanup(func() { listenAndServe = restoreListen })

			err := runServer(context.Background(), testServerConfig(t), s3store.Config{})
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
	newMediaStore = func(context.Context, s3store.Config) (readyObjectStore, error) {
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

	if err := runServer(context.Background(), testServerConfig(t), s3store.Config{}); err != nil {
		t.Fatalf("run server: %v", err)
	}
	if !listened {
		t.Fatal("server was not asked to listen")
	}
}

func TestRunServerCleansExpiredUploadsBeforeListen(t *testing.T) {
	cfg := testServerConfig(t)
	seedExpiredUpload(t, cfg.databasePath)

	var events []string
	restoreMediaStoreFactory(t)
	newMediaStore = func(context.Context, s3store.Config) (readyObjectStore, error) {
		return fakeMediaReadiness{
			verify: func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("readiness context has no deadline")
				}
				events = append(events, "readiness")
				return nil
			},
			delete: func(ctx context.Context, _ media.ObjectKey) error {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("cleanup context has no deadline")
				}
				if remaining := time.Until(deadline); remaining <= 0 || remaining > startupReadinessTimeout {
					t.Fatalf("cleanup deadline remaining = %s, want >0 and <= %s", remaining, startupReadinessTimeout)
				}
				events = append(events, "cleanup")
				return nil
			},
		}, nil
	}

	restoreListen := listenAndServe
	listenAndServe = func(*http.Server) error {
		events = append(events, "listen")
		return http.ErrServerClosed
	}
	t.Cleanup(func() { listenAndServe = restoreListen })

	if err := runServer(context.Background(), cfg, s3store.Config{}); err != nil {
		t.Fatalf("run server: %v", err)
	}
	if diff := cmp.Diff([]string{"readiness", "cleanup", "listen"}, events); diff != "" {
		t.Fatalf("startup order mismatch (-want +got):\n%s", diff)
	}
}

func TestRunServerCleanupFailurePreventsListenAndClosesDatabase(t *testing.T) {
	cfg := testServerConfig(t)
	seedExpiredUpload(t, cfg.databasePath)
	cleanupErr := errors.New("cleanup failed")

	restoreMediaStoreFactory(t)
	newMediaStore = func(context.Context, s3store.Config) (readyObjectStore, error) {
		return fakeMediaReadiness{
			verify: func(context.Context) error { return nil },
			delete: func(context.Context, media.ObjectKey) error { return cleanupErr },
		}, nil
	}

	listened := false
	restoreListen := listenAndServe
	listenAndServe = func(*http.Server) error {
		listened = true
		return http.ErrServerClosed
	}
	t.Cleanup(func() { listenAndServe = restoreListen })

	closed := false
	restoreClose := closeDatabase
	closeDatabase = func(database *sql.DB) error {
		closed = true
		return database.Close()
	}
	t.Cleanup(func() { closeDatabase = restoreClose })

	err := runServer(context.Background(), cfg, s3store.Config{})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("run server error = %v, want cleanup error", err)
	}
	if !errors.Is(err, media.ErrMediaStorageUnavailable) {
		t.Fatalf("run server error = %v, want media storage unavailable", err)
	}
	if listened {
		t.Fatal("server listened after cleanup failure")
	}
	if !closed {
		t.Fatal("database was not closed after cleanup failure")
	}
}

type fakeMediaReadiness struct {
	verify func(context.Context) error
	delete func(context.Context, media.ObjectKey) error
}

func (fake fakeMediaReadiness) VerifyReadiness(ctx context.Context) error {
	if fake.verify == nil {
		return errors.New("unexpected media readiness")
	}
	return fake.verify(ctx)
}

func (fakeMediaReadiness) Put(context.Context, media.PutObject) error {
	return errors.New("unexpected media Put")
}

func (fakeMediaReadiness) Open(context.Context, media.ObjectKey) (media.Object, error) {
	return media.Object{}, errors.New("unexpected media Open")
}

func (fake fakeMediaReadiness) Delete(ctx context.Context, key media.ObjectKey) error {
	if fake.delete == nil {
		return errors.New("unexpected media Delete")
	}
	return fake.delete(ctx, key)
}

func restoreMediaStoreFactory(t *testing.T) {
	t.Helper()
	original := newMediaStore
	t.Cleanup(func() { newMediaStore = original })
}

func restoreS3ConfigLoader(t *testing.T) {
	t.Helper()
	original := loadS3Config
	t.Cleanup(func() { loadS3Config = original })
}

func seedExpiredUpload(t *testing.T, databasePath string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open cleanup database: %v", err)
	}
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		_ = db.Close()
		t.Fatalf("apply cleanup migrations: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	const userID = "startup-cleanup-user"
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', ?, ?)`, userID, now.UnixMilli(), now.UnixMilli()); err != nil {
		_ = db.Close()
		t.Fatalf("insert cleanup user: %v", err)
	}
	const uploadID = "123e4567-e89b-12d3-a456-426614174000"
	store := sqlite.NewImageUploadStore(db)
	_, err = store.BeginPending(ctx, media.PendingInput{
		ID:                    uploadID,
		UserID:                userID,
		StorageKey:            media.ObjectKey("note-images/" + uploadID),
		UploadRequestID:       "123e4567-e89b-12d3-a456-426614174001",
		ContentType:           "image/jpeg",
		ByteSize:              1,
		Width:                 1,
		Height:                1,
		SHA256:                strings.Repeat("a", 64),
		CreatedAt:             now.Add(-2 * time.Hour),
		UpdatedAt:             now.Add(-2 * time.Hour),
		WriteLeaseUntil:       now.Add(time.Minute),
		ExpiresAt:             now.Add(time.Hour),
		RequestRetentionUntil: now.Add(48 * time.Hour),
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("insert pending cleanup upload: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE image_uploads SET expires_at = ?, write_lease_until = NULL WHERE id = ?`, now.Add(-time.Second).UnixMilli(), uploadID); err != nil {
		_ = db.Close()
		t.Fatalf("expire cleanup upload: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close cleanup seed database: %v", err)
	}
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

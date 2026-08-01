package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tprei/sdds/services/api/internal/embedding"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/s3store"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

func captureReindexOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	original := reindexOutputStream
	buf := &bytes.Buffer{}
	reindexOutputStream = buf
	t.Cleanup(func() { reindexOutputStream = original })
	return buf
}

func seedReindexDatabase(t *testing.T, databasePath string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(databasePath)
	if err != nil {
		t.Fatalf("open seed database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close seed database: %v", err)
		}
	}()
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	const userID = "reindex-user"
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO notes (id, user_id, title, body, category_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"reindex-note", userID, "Nota sem vetor", "Precisa de reindex.", note.CategorySlugFood, 0, 0,
	); err != nil {
		t.Fatalf("insert note: %v", err)
	}
}

func fakeEmbeddingClientFactory(fake fakeEmbeddingReadiness) func(embedding.Config) (embeddingStore, error) {
	return func(embedding.Config) (embeddingStore, error) {
		return fake, nil
	}
}

func TestRunReindexEmbeddingsOutputsSummary(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedReindexDatabase(t, databasePath)

	restoreEmbeddingClientFactory(t)
	newEmbeddingClient = fakeEmbeddingClientFactory(fakeEmbeddingReadiness{
		verify: func(context.Context) error { return nil },
		embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
			vectors := make([][]float32, len(texts))
			for i := range vectors {
				vector := make([]float32, note.EmbeddingDimension)
				vector[0] = 1
				vectors[i] = vector
			}
			return vectors, nil
		},
	})
	output := captureReindexOutput(t)

	if err := runReindexEmbeddings(context.Background(), config{databasePath: databasePath}, embedding.Config{}); err != nil {
		t.Fatalf("run reindex embeddings: %v", err)
	}

	var decoded reindexOutputRow
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode reindex summary: %v\noutput: %s", err, output.String())
	}
	if decoded != (reindexOutputRow{Scanned: 1, Embedded: 1, Skipped: 0}) {
		t.Fatalf("summary = %+v, want {Scanned:1 Embedded:1 Skipped:0}", decoded)
	}
}

func TestRunReindexEmbeddingsIsIdempotent(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedReindexDatabase(t, databasePath)

	restoreEmbeddingClientFactory(t)
	newEmbeddingClient = fakeEmbeddingClientFactory(fakeEmbeddingReadiness{
		verify: func(context.Context) error { return nil },
		embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
			vectors := make([][]float32, len(texts))
			for i := range vectors {
				vector := make([]float32, note.EmbeddingDimension)
				vector[0] = 1
				vectors[i] = vector
			}
			return vectors, nil
		},
	})

	first := captureReindexOutput(t)
	if err := runReindexEmbeddings(context.Background(), config{databasePath: databasePath}, embedding.Config{}); err != nil {
		t.Fatalf("first reindex: %v", err)
	}
	var firstResult reindexOutputRow
	if err := json.Unmarshal(first.Bytes(), &firstResult); err != nil {
		t.Fatalf("decode first summary: %v", err)
	}
	if firstResult.Embedded != 1 {
		t.Fatalf("first embedded = %d, want 1", firstResult.Embedded)
	}

	second := captureReindexOutput(t)
	if err := runReindexEmbeddings(context.Background(), config{databasePath: databasePath}, embedding.Config{}); err != nil {
		t.Fatalf("second reindex: %v", err)
	}
	var secondResult reindexOutputRow
	if err := json.Unmarshal(second.Bytes(), &secondResult); err != nil {
		t.Fatalf("decode second summary: %v", err)
	}
	if secondResult.Embedded != 0 {
		t.Fatalf("second embedded = %d, want 0 (idempotent)", secondResult.Embedded)
	}
	if secondResult.Skipped != 1 {
		t.Fatalf("second skipped = %d, want 1", secondResult.Skipped)
	}
}

func TestRunReindexEmbeddingsRequiresEmbeddingReadiness(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sdds.db")
	seedReindexDatabase(t, databasePath)

	readinessErr := errors.New("sidecar unreachable")
	restoreEmbeddingClientFactory(t)
	newEmbeddingClient = fakeEmbeddingClientFactory(fakeEmbeddingReadiness{
		verify: func(context.Context) error { return readinessErr },
	})
	captureReindexOutput(t)

	err := runReindexEmbeddings(context.Background(), config{databasePath: databasePath}, embedding.Config{})
	if !errors.Is(err, readinessErr) {
		t.Fatalf("run reindex embeddings error = %v, want %v", err, readinessErr)
	}
}

func TestRunLoadsEmbeddingConfigForReindexCommand(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SDDS_DATABASE_PATH", filepath.Join(t.TempDir(), "sdds.db"))
	originalArgs := os.Args
	os.Args = []string{"api", commandReindexEmbeddings}
	t.Cleanup(func() { os.Args = originalArgs })

	restoreS3ConfigLoader(t)
	s3Loaded := false
	loadS3Config = func() (s3store.Config, error) {
		s3Loaded = true
		return s3store.Config{}, nil
	}

	restoreEmbeddingConfigLoader(t)
	embeddingLoaded := false
	loadEmbeddingConfig = func() (embedding.Config, error) {
		embeddingLoaded = true
		return embedding.Config{}, nil
	}

	restoreEmbeddingClientFactory(t)
	newEmbeddingClient = fakeEmbeddingClientFactory(fakeEmbeddingReadiness{
		verify: func(context.Context) error { return nil },
		embedPassages: func(_ context.Context, texts []string) ([][]float32, error) {
			return make([][]float32, len(texts)), nil
		},
	})
	captureReindexOutput(t)

	if err := run(); err != nil {
		t.Fatalf("run reindex-embeddings command: %v", err)
	}
	if !s3Loaded {
		t.Fatal("reindex-embeddings did not load S3 config")
	}
	if !embeddingLoaded {
		t.Fatal("reindex-embeddings did not load embedding config")
	}
}

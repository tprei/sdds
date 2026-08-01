package embedding

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

func testConfig(t *testing.T, url string) Config {
	t.Helper()
	return Config{
		url:          url,
		queryTimeout: time.Second,
		batchTimeout: time.Second,
		loaded:       true,
	}
}

func unitVector(dimension int) []float32 {
	values := make([]float32, dimension)
	for i := range values {
		values[i] = 1
	}
	var sumSquares float64
	for _, v := range values {
		sumSquares += float64(v) * float64(v)
	}
	scale := float32(1 / math.Sqrt(sumSquares))
	for i := range values {
		values[i] *= scale
	}
	return values
}

func TestClientEmbedQueryHappyPath(t *testing.T) {
	vector := unitVector(note.EmbeddingDimension)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %q, want /v1/embeddings", r.URL.Path)
		}
		var decoded embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(decoded.Texts) != 1 || decoded.Texts[0] != "cafe bom" {
			t.Fatalf("texts = %+v, want [cafe bom]", decoded.Texts)
		}
		_ = json.NewEncoder(w).Encode(embeddingsResponse{
			ModelID:       note.EmbeddingModelID,
			ModelRevision: note.EmbeddingModelRevision,
			Dimension:     note.EmbeddingDimension,
			Vectors:       [][]float32{vector},
		})
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	got, err := client.EmbedQuery(context.Background(), "cafe bom")
	if err != nil {
		t.Fatalf("embed query: %v", err)
	}
	if len(got) != note.EmbeddingDimension {
		t.Fatalf("vector length = %d, want %d", len(got), note.EmbeddingDimension)
	}
}

func TestClientEmbedQueryRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embeddingsResponse{
			ModelID:       note.EmbeddingModelID,
			ModelRevision: note.EmbeddingModelRevision,
			Dimension:     note.EmbeddingDimension,
			Vectors:       [][]float32{make([]float32, note.EmbeddingDimension-1)},
		})
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.EmbedQuery(context.Background(), "cafe"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("embed query error = %v, want ErrUnavailable", err)
	}
}

func TestClientEmbedQueryRejectsUnnormalizedVector(t *testing.T) {
	vector := make([]float32, note.EmbeddingDimension)
	vector[0] = 5 // norm 5, far outside tolerance of 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embeddingsResponse{
			ModelID:       note.EmbeddingModelID,
			ModelRevision: note.EmbeddingModelRevision,
			Dimension:     note.EmbeddingDimension,
			Vectors:       [][]float32{vector},
		})
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.EmbedQuery(context.Background(), "cafe"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("embed query error = %v, want ErrUnavailable", err)
	}
}

func TestClientEmbedQueryRejectsNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.EmbedQuery(context.Background(), "cafe"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("embed query error = %v, want ErrUnavailable", err)
	}
}

func TestClientEmbedPassagesRejectsOversizedBatch(t *testing.T) {
	client, err := New(testConfig(t, "http://example.invalid"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	texts := make([]string, maxBatchSize+1)
	if _, err := client.EmbedPassages(context.Background(), texts); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("embed passages error = %v, want ErrUnavailable", err)
	}
}

func TestClientVerifyReadinessAcceptsMatchingModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(healthzResponse{
			Status:        "ok",
			ModelID:       note.EmbeddingModelID,
			ModelRevision: note.EmbeddingModelRevision,
			Dimension:     note.EmbeddingDimension,
		})
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.VerifyReadiness(context.Background()); err != nil {
		t.Fatalf("verify readiness: %v", err)
	}
}

func TestClientVerifyReadinessRejectsMismatchedRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(healthzResponse{
			Status:        "ok",
			ModelID:       note.EmbeddingModelID,
			ModelRevision: "some-other-revision",
			Dimension:     note.EmbeddingDimension,
		})
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.VerifyReadiness(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("verify readiness error = %v, want ErrUnavailable", err)
	}
}

func TestClientVerifyReadinessRejectsServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.VerifyReadiness(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("verify readiness error = %v, want ErrUnavailable", err)
	}
}

func TestClientEmbedQueryRespectsContextTimeout(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer server.Close()
	defer close(blocked)

	config := testConfig(t, server.URL)
	config.queryTimeout = 10 * time.Millisecond
	client, err := New(config)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.EmbedQuery(context.Background(), "cafe"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("embed query error = %v, want ErrUnavailable", err)
	}
}

func TestNewRejectsUnloadedConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for unloaded config, got nil")
	}
}

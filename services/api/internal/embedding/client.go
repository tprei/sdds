package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
)

// ErrUnavailable wraps every failure mode of the embedding sidecar: transport
// errors, non-2xx responses, and a response that fails vector validation.
// Callers never distinguish those causes -- from the domain's perspective the
// embedding runtime is simply unavailable and there is no fallback mode.
var ErrUnavailable = errors.New("embedding runtime unavailable")

const maxBatchSize = 32

// normTolerance is how far a returned vector's L2 norm may drift from 1
// before it is rejected. The sidecar normalizes; this catches a broken or
// mismatched export rather than tightening ranking precision.
const normTolerance = 1e-3

type Client struct {
	httpClient   *http.Client
	baseURL      string
	queryTimeout time.Duration
	batchTimeout time.Duration
}

// New builds a Client from a loaded Config. It performs no network I/O; the
// sidecar is reached only by EmbedQuery, EmbedPassages, and VerifyReadiness.
func New(config Config) (*Client, error) {
	if !config.loaded {
		return nil, errors.New("embedding config is not loaded")
	}
	return &Client{
		httpClient:   &http.Client{},
		baseURL:      config.url,
		queryTimeout: config.queryTimeout,
		batchTimeout: config.batchTimeout,
	}, nil
}

type embeddingsRequest struct {
	Texts []string `json:"texts"`
}

type embeddingsResponse struct {
	ModelID       string      `json:"model_id"`
	ModelRevision string      `json:"model_revision"`
	Dimension     int         `json:"dimension"`
	Vectors       [][]float32 `json:"vectors"`
}

type healthzResponse struct {
	Status        string `json:"status"`
	ModelID       string `json:"model_id"`
	ModelRevision string `json:"model_revision"`
	Dimension     int    `json:"dimension"`
}

// EmbedQuery embeds a single search query text.
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vectors, err := c.embed(ctx, c.queryTimeout, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

// EmbedPassages embeds a bounded batch of note passages.
func (c *Client) EmbedPassages(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) > maxBatchSize {
		return nil, fmt.Errorf("%w: batch of %d exceeds max %d", ErrUnavailable, len(texts), maxBatchSize)
	}
	return c.embed(ctx, c.batchTimeout, texts)
}

func (c *Client) embed(ctx context.Context, timeout time.Duration, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(embeddingsRequest{Texts: texts})
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %v", ErrUnavailable, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request: %v", ErrUnavailable, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrUnavailable, response.StatusCode)
	}

	var decoded embeddingsResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrUnavailable, err)
	}
	if len(decoded.Vectors) != len(texts) {
		return nil, fmt.Errorf("%w: got %d vectors for %d texts", ErrUnavailable, len(decoded.Vectors), len(texts))
	}
	for _, vector := range decoded.Vectors {
		if err := validateVector(vector); err != nil {
			return nil, err
		}
	}
	return decoded.Vectors, nil
}

func validateVector(vector []float32) error {
	if len(vector) != note.EmbeddingDimension {
		return fmt.Errorf("%w: vector dimension %d, want %d", ErrUnavailable, len(vector), note.EmbeddingDimension)
	}
	var sumSquares float64
	for _, value := range vector {
		sumSquares += float64(value) * float64(value)
	}
	norm := math.Sqrt(sumSquares)
	if math.Abs(norm-1) > normTolerance {
		return fmt.Errorf("%w: vector norm %f is not unit-length within %g", ErrUnavailable, norm, normTolerance)
	}
	return nil
}

// VerifyReadiness confirms the sidecar has loaded the pinned production model.
// A sidecar serving a different model id, revision, or dimension fails
// readiness rather than silently serving mismatched vectors.
func (c *Client) VerifyReadiness(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("%w: build readiness request: %v", ErrUnavailable, err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: readiness request: %v", ErrUnavailable, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: readiness status %d", ErrUnavailable, response.StatusCode)
	}
	var decoded healthzResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return fmt.Errorf("%w: decode readiness response: %v", ErrUnavailable, err)
	}
	if decoded.ModelID != note.EmbeddingModelID {
		return fmt.Errorf("%w: sidecar model id %q, want %q", ErrUnavailable, decoded.ModelID, note.EmbeddingModelID)
	}
	if decoded.ModelRevision != note.EmbeddingModelRevision {
		return fmt.Errorf("%w: sidecar model revision %q, want %q", ErrUnavailable, decoded.ModelRevision, note.EmbeddingModelRevision)
	}
	if decoded.Dimension != note.EmbeddingDimension {
		return fmt.Errorf("%w: sidecar dimension %d, want %d", ErrUnavailable, decoded.Dimension, note.EmbeddingDimension)
	}
	return nil
}

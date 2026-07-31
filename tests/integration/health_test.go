//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// TestHealthRuntimeBoundaries proves the Compose-to-runtime boundary: the
// assembled image reaches readiness and serves GET /healthz with no content.
func TestHealthRuntimeBoundaries(t *testing.T) {
	client := newAPIClient(t)
	waitForReadiness(t, client)

	health, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	requireStatus(t, "GET /healthz", health.StatusCode(), http.StatusNoContent, health.Body)
}

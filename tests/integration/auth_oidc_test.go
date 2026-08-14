//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestOIDCSessionDisabledRuntimeBoundaries proves the Compose-to-runtime
// contract of provider sign-in when the assembled stack runs with provider
// sign-in turned off, which is the default: the endpoint exists on the router
// and answers 503 oidc_unavailable rather than attempting verification, and it
// stays consistent across repeated attempts.
func TestOIDCSessionDisabledRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	body := openapi.CreateAuthOidcSessionJSONRequestBody{
		Provider: "google",
		IdToken:  "not-a-real-token",
		Nonce:    "nonce",
	}
	for attempt := 1; attempt <= 3; attempt++ {
		response, err := publicClient.CreateAuthOidcSessionWithResponse(context.Background(), body)
		if err != nil {
			t.Fatalf("attempt %d POST /v1/auth/oidc/sessions: %v", attempt, err)
		}
		requireStatus(t, fmt.Sprintf("attempt %d POST /v1/auth/oidc/sessions", attempt), response.StatusCode(), http.StatusServiceUnavailable, response.Body)
		if response.JSON503 == nil {
			t.Fatalf("attempt %d: 503 body is absent: %s", attempt, string(response.Body))
		}
		if response.JSON503.Code != openapi.ErrorCodeOidcUnavailable {
			t.Fatalf("attempt %d: code = %s, want %s", attempt, response.JSON503.Code, openapi.ErrorCodeOidcUnavailable)
		}
		var raw openapi.ErrorResponse
		if err := json.Unmarshal(response.Body, &raw); err != nil {
			t.Fatalf("attempt %d: decode body: %v", attempt, err)
		}
		if raw.Fields != nil && len(*raw.Fields) > 0 {
			t.Fatalf("attempt %d: fields = %+v, want none", attempt, raw.Fields)
		}
	}
}

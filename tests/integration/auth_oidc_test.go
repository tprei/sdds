//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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

// TestLoginIdentitiesRuntimeBoundaries proves the identity surfaces against
// the assembled stack: the session response carries the caller's login
// identities, the only identity cannot be disconnected, and a foreign or
// unknown identity answers 404.
func TestLoginIdentitiesRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	signup := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("ident-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Identidade",
	})
	client := newAuthenticatedAPIClient(t, signup.Token)

	session, err := client.GetAuthSessionWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /v1/auth/session: %v", err)
	}
	requireStatus(t, "GET /v1/auth/session", session.StatusCode(), http.StatusOK, session.Body)
	if session.JSON200 == nil {
		t.Fatalf("session body is absent: %s", string(session.Body))
	}
	identities := session.JSON200.User.Identities
	if len(identities) != 1 {
		t.Fatalf("identities = %+v, want exactly the password identity", identities)
	}
	if identities[0].Kind != openapi.LoginIdentityKindPassword || identities[0].Provider != openapi.LoginIdentityProviderLocal {
		t.Fatalf("identity = %+v, want password/local", identities[0])
	}

	last, err := client.DeleteAuthIdentityWithResponse(context.Background(), identities[0].Id)
	if err != nil {
		t.Fatalf("DELETE /v1/auth/identities/{id}: %v", err)
	}
	requireStatus(t, "DELETE last identity", last.StatusCode(), http.StatusConflict, last.Body)
	if last.JSON409 == nil || last.JSON409.Code != openapi.ErrorCodeLastSignInMethod {
		t.Fatalf("DELETE last identity body = %s, want last_sign_in_method", string(last.Body))
	}

	missing, err := client.DeleteAuthIdentityWithResponse(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("DELETE unknown identity: %v", err)
	}
	requireStatus(t, "DELETE unknown identity", missing.StatusCode(), http.StatusNotFound, missing.Body)
}

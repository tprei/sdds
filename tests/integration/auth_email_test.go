//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestAuthEmailAndPasswordRecoveryRuntimeBoundaries proves the Compose-to-runtime
// verification and password-recovery boundary against a captured mail sink: an
// emailed verification token verifies an address, a recovery token resets the
// password and revokes the prior session, and the new password logs in.
func TestAuthEmailAndPasswordRecoveryRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	username := fmt.Sprintf("rec-%d", time.Now().UnixNano())
	address := fmt.Sprintf("%s@sdds.test", username)
	password := "secret-password"
	newPassword := "nova-secret-password"

	signup := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    password,
		DisplayName: "Recovery",
		Email:       &address,
	})
	authedClient := newAuthenticatedAPIClient(t, signup.Token)

	t.Run("verification token verifies the email", func(t *testing.T) {
		verifyToken := waitForEmailToken(t, address, "verify-email")

		response, err := authedClient.VerifyAuthEmailWithResponse(context.Background(), openapi.VerifyAuthEmailJSONRequestBody{Token: verifyToken})
		if err != nil {
			t.Fatalf("POST /v1/auth/email/verification: %v", err)
		}
		requireStatus(t, "POST /v1/auth/email/verification", response.StatusCode(), http.StatusNoContent, response.Body)

		replay, err := authedClient.VerifyAuthEmailWithResponse(context.Background(), openapi.VerifyAuthEmailJSONRequestBody{Token: verifyToken})
		if err != nil {
			t.Fatalf("replay verification: %v", err)
		}
		requireStatus(t, "replay verification", replay.StatusCode(), http.StatusBadRequest, replay.Body)
	})

	t.Run("recovery token resets the password and revokes the session", func(t *testing.T) {
		resetResponse, err := publicClient.CreateAuthPasswordResetWithResponse(context.Background(), openapi.CreateAuthPasswordResetJSONRequestBody{Email: address})
		if err != nil {
			t.Fatalf("POST /v1/auth/password-resets: %v", err)
		}
		requireStatus(t, "POST /v1/auth/password-resets", resetResponse.StatusCode(), http.StatusAccepted, resetResponse.Body)

		resetToken := waitForEmailToken(t, address, "new-password")

		setResponse, err := publicClient.SetAuthPasswordWithResponse(context.Background(), openapi.SetAuthPasswordJSONRequestBody{Token: resetToken, Password: newPassword})
		if err != nil {
			t.Fatalf("POST /v1/auth/password: %v", err)
		}
		requireStatus(t, "POST /v1/auth/password", setResponse.StatusCode(), http.StatusNoContent, setResponse.Body)

		// The pre-reset session is revoked.
		sessionResponse, err := authedClient.GetAuthSessionWithResponse(context.Background())
		if err != nil {
			t.Fatalf("GET /v1/auth/session after reset: %v", err)
		}
		requireStatus(t, "GET /v1/auth/session after reset", sessionResponse.StatusCode(), http.StatusUnauthorized, sessionResponse.Body)

		// The new password logs in.
		loggedIn := createAuthSession(t, publicClient, openapi.CreateAuthSessionJSONRequestBody{
			Username: username,
			Password: newPassword,
		})
		requireAuthSession(t, loggedIn, username, "Recovery")
	})
}

// mailSinkMessage is the captured message shape returned by the test sink.
type mailSinkMessage struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
}

// waitForEmailToken polls the mail sink for a message addressed to the given
// recipient whose embedded link matches linkPath, and extracts its token.
func waitForEmailToken(t *testing.T, address string, linkPath string) string {
	t.Helper()
	sinkURL := os.Getenv("SDDS_MAILSINK_URL")
	if sinkURL == "" {
		sinkURL = "http://127.0.0.1:8090"
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		token := readTokenFromSink(t, sinkURL, address, linkPath)
		if token != "" {
			return token
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no captured %s email for %s within 15s", linkPath, address)
	return ""
}

func readTokenFromSink(t *testing.T, sinkURL string, address string, linkPath string) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, sinkURL+"/messages?to="+address, nil)
	if err != nil {
		t.Fatalf("build sink request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("query mail sink: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var payload struct {
		Messages []mailSinkMessage `json:"messages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode sink response: %v", err)
	}
	for _, msg := range payload.Messages {
		for _, to := range msg.To {
			if !strings.EqualFold(to, address) {
				continue
			}
			if token := extractToken(msg.Text, linkPath); token != "" {
				return token
			}
			if token := extractToken(msg.HTML, linkPath); token != "" {
				return token
			}
		}
	}
	return ""
}

func extractToken(body string, linkPath string) string {
	for _, marker := range []string{linkPath + "?token=", linkPath + "%3Ftoken%3D"} {
		if index := strings.Index(body, marker); index >= 0 {
			start := index + len(marker)
			rest := body[start:]
			end := strings.IndexAny(rest, "&\"< \n\r")
			if end < 0 {
				end = len(rest)
			}
			return rest[:end]
		}
	}
	return ""
}

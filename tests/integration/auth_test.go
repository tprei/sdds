//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestAuthRuntimeBoundaries proves the Compose-to-runtime auth boundary:
// signup, duplicate rejection, current-session read, logout, invalid-login
// rejection, and re-login.
func TestAuthRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	username := fmt.Sprintf("thiago-%d", time.Now().UnixNano())
	displayName := "Thiago Integração"
	password := "secret-password"
	createdSession := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	})
	requireAuthSession(t, createdSession, username, displayName)
	requireDuplicateAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	})
	sessionClient := newAuthenticatedAPIClient(t, createdSession.Token)
	currentSession := getAuthSession(t, sessionClient)
	requireCurrentSession(t, currentSession, createdSession)
	deleteAuthSession(t, sessionClient)
	requireUnauthenticatedAuthSession(t, sessionClient)
	requireInvalidAuthSession(t, publicClient, username, "wrong-password")
	loggedInSession := createAuthSession(t, publicClient, openapi.CreateAuthSessionJSONRequestBody{
		Username: username,
		Password: password,
	})
	requireAuthSession(t, loggedInSession, username, displayName)
	requireCurrentSession(t, getAuthSession(t, newAuthenticatedAPIClient(t, loggedInSession.Token)), loggedInSession)
}

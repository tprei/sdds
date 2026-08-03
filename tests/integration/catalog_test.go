//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestCatalogRuntimeBoundaries proves the Compose-to-runtime boundary for the
// public category catalog served from the migrated SQLite catalog seed.
func TestCatalogRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("catalog-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Catálogo Runtime",
	})
	client := newAuthenticatedAPIClient(t, session.Token)
	requireCatalogs(t, client)
}

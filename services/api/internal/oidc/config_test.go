package oidc

import (
	"errors"
	"strings"
	"testing"
)

func clearOIDCEnv(t *testing.T) {
	t.Helper()
	t.Setenv(modeEnv, "")
	t.Setenv(googleAudiencesEnv, "")
	t.Setenv(appleAudiencesEnv, "")
}

func TestLoadFromEnvDefaultsToDisabled(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, "")
	client, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if client != nil {
		t.Fatal("LoadFromEnv() returned a client while mode was unset")
	}
}

func TestLoadFromEnvDisabledCaseInsensitiveAndTrimmed(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, "  DiSaBlEd ")
	client, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if client != nil {
		t.Fatal("LoadFromEnv() returned a client while mode was disabled")
	}
}

func TestLoadFromEnvRejectsInvalidMode(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, "on")
	_, err := LoadFromEnv()
	if err == nil || err.Error() != `SDDS_AUTH_OIDC_MODE must be "enabled" or "disabled"` {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
}

func TestLoadFromEnvLoadsTrimmedAudienceLists(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, " ENABLED ")
	t.Setenv(googleAudiencesEnv, " google-app , second-app ")
	t.Setenv(appleAudiencesEnv, "apple-app")

	client, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if client == nil {
		t.Fatal("LoadFromEnv() returned nil client")
	}
	if got := client.providers[ProviderGoogle].audiences; len(got) != 2 || got[0] != "google-app" || got[1] != "second-app" {
		t.Fatalf("Google audiences = %#v", got)
	}
	if got := client.providers[ProviderApple].audiences; len(got) != 1 || got[0] != "apple-app" {
		t.Fatalf("Apple audiences = %#v", got)
	}
}

func TestLoadFromEnvAllowsOneProviderAudienceList(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, modeEnabled)
	t.Setenv(googleAudiencesEnv, "google-app")

	client, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if client == nil || len(client.providers) != 1 {
		t.Fatalf("client/providers = %#v/%d", client, len(client.providers))
	}
	if _, ok := client.providers[ProviderApple]; ok {
		t.Fatal("Apple provider was registered without an audience list")
	}
}

func TestLoadFromEnvRejectsEnabledWithNoProviders(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, modeEnabled)
	_, err := LoadFromEnv()
	if err == nil || err.Error() != "SDDS_AUTH_OIDC_MODE=enabled requires at least one provider audience list" {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
}

func TestLoadFromEnvRejectsMalformedAudienceLists(t *testing.T) {
	for _, raw := range []string{" ", "google-app,", ",google-app", "google-app,,other"} {
		t.Run(strings.ReplaceAll(raw, ",", "_"), func(t *testing.T) {
			clearOIDCEnv(t)
			t.Setenv(modeEnv, modeEnabled)
			t.Setenv(googleAudiencesEnv, raw)
			_, err := LoadFromEnv()
			if err == nil || err.Error() != "SDDS_AUTH_OIDC_GOOGLE_AUDIENCES must be a comma-separated list of audiences" {
				t.Fatalf("LoadFromEnv() error = %v", err)
			}
		})
	}
}

func TestLoadFromEnvRejectsMalformedAppleAudienceLists(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, modeEnabled)
	t.Setenv(appleAudiencesEnv, "apple-app, ")
	_, err := LoadFromEnv()
	if err == nil || err.Error() != "SDDS_AUTH_OIDC_APPLE_AUDIENCES must be a comma-separated list of audiences" {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
}

func TestLoadFromEnvErrorsAreNotUnavailable(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv(modeEnv, modeEnabled)
	_, err := LoadFromEnv()
	if errors.Is(err, ErrUnavailable) {
		t.Fatal("configuration error was classified as provider unavailability")
	}
}

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/httpapi"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	clearConfigEnv(t)

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := config{
		authLimits:   httpapi.DefaultAuthLimits(),
		databasePath: defaultDatabasePath,
		httpAddr:     defaultHTTPAddr,
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(config{})); diff != "" {
		t.Fatalf("config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigUsesEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SDDS_DATABASE_PATH", "/tmp/sdds-test.db")
	t.Setenv("SDDS_HTTP_ADDR", "127.0.0.1:18080")
	t.Setenv("SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", "7")
	t.Setenv("SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", "11")
	t.Setenv("SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", "70")
	t.Setenv("SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", "110")
	t.Setenv("SDDS_AUTH_HASH_CONCURRENCY", "3")

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := config{
		authLimits: httpapi.AuthLimits{
			SignupRequestsPerMinute:       7,
			LoginRequestsPerMinute:        11,
			SignupGlobalRequestsPerMinute: 70,
			LoginGlobalRequestsPerMinute:  110,
			PasswordHashConcurrency:       3,
		},
		databasePath: "/tmp/sdds-test.db",
		httpAddr:     "127.0.0.1:18080",
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(config{})); diff != "" {
		t.Fatalf("config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfigRejectsInvalidAuthLimits(t *testing.T) {
	tests := []struct {
		name    string
		envName string
		value   string
	}{
		{name: "signup malformed", envName: "SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", value: "often"},
		{name: "signup zero", envName: "SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", value: "0"},
		{name: "login negative", envName: "SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", value: "-1"},
		{name: "global signup zero", envName: "SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", value: "0"},
		{name: "global login negative", envName: "SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", value: "-1"},
		{name: "hash concurrency malformed", envName: "SDDS_AUTH_HASH_CONCURRENCY", value: "many"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv(tt.envName, tt.value)

			_, err := loadConfig()
			if err == nil {
				t.Fatal("load config error is nil")
			}
			want := tt.envName + " must be a positive integer"
			if got := err.Error(); got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SDDS_DATABASE_PATH", "")
	t.Setenv("SDDS_HTTP_ADDR", "")
	t.Setenv("SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", "")
	t.Setenv("SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", "")
	t.Setenv("SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", "")
	t.Setenv("SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", "")
	t.Setenv("SDDS_AUTH_HASH_CONCURRENCY", "")
}

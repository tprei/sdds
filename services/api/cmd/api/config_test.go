package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
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

func TestLoadMediaConfigUsesEnvironment(t *testing.T) {
	clearConfigEnv(t)
	accessKeyFile, secretKeyFile := setValidMediaEnv(t)
	got, err := loadMediaConfig()
	if err != nil {
		t.Fatalf("load media config: %v", err)
	}

	want := media.Config{
		Endpoint: "http://rustfs:9000/", Region: "us-east-1", Bucket: "sdds-media", UsePathStyle: true,
		AccessKeyFile: accessKeyFile, SecretKeyFile: secretKeyFile,
		Timeout: mediaRequestTimeout, RetryMaxAttempts: mediaRetryMaxAttempts,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("media config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadMediaConfigRejectsInvalidValues(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name, env, value, want, prefix, fileContents string
		keepEnv                                      bool
	}{
		{"endpoint missing", mediaEndpointEnv, "", mediaEndpointEnv + " is required", "", "", false},
		{"region missing", mediaRegionEnv, "", mediaRegionEnv + " is required", "", "", false},
		{"bucket missing", mediaBucketEnv, "", mediaBucketEnv + " is required", "", "", false},
		{"path style missing", mediaPathStyleEnv, "", mediaPathStyleEnv + " is required", "", "", false},
		{"access key file missing", mediaAccessKeyFileEnv, "", mediaAccessKeyFileEnv + " is required", "", "", false},
		{"secret key file missing", mediaSecretKeyFileEnv, "", mediaSecretKeyFileEnv + " is required", "", "", false},
		{"endpoint syntax", mediaEndpointEnv, "://rustfs:9000", mediaEndpointEnv + " must be a valid HTTP(S) URL", "", "", false},
		{"endpoint scheme", mediaEndpointEnv, "ftp://rustfs:9000", mediaEndpointEnv + " must use HTTP or HTTPS", "", "", false},
		{"endpoint insecure remote", mediaEndpointEnv, "http://localhost:9000", mediaEndpointEnv + " must use HTTPS except for http://rustfs:9000", "", "", false},
		{"endpoint insecure lookalike", mediaEndpointEnv, "http://rustfs.example:9000", mediaEndpointEnv + " must use HTTPS except for http://rustfs:9000", "", "", false},
		{"endpoint wrong rustfs port", mediaEndpointEnv, "http://rustfs:9001", mediaEndpointEnv + " must use HTTPS except for http://rustfs:9000", "", "", false},
		{"endpoint insecure rustfs path", mediaEndpointEnv, "http://rustfs:9000/base", mediaEndpointEnv + " must use HTTPS except for http://rustfs:9000", "", "", false},
		{"endpoint credentials", mediaEndpointEnv, "https://user:pass@example.com", mediaEndpointEnv + " must be a valid HTTP(S) URL", "", "", false},
		{"endpoint query", mediaEndpointEnv, "https://example.com?x=1", mediaEndpointEnv + " must be a valid HTTP(S) URL", "", "", false},
		{"endpoint fragment", mediaEndpointEnv, "https://example.com#fragment", mediaEndpointEnv + " must be a valid HTTP(S) URL", "", "", false},
		{"path style invalid", mediaPathStyleEnv, "yes", mediaPathStyleEnv + " must be true or false", "", "", false},
		{"access key unreadable", mediaAccessKeyFileEnv, missingFile, "", "read " + mediaAccessKeyFileEnv + ":", "", false},
		{"secret key unreadable", mediaSecretKeyFileEnv, missingFile, "", "read " + mediaSecretKeyFileEnv + ":", "", false},
		{"access key empty", mediaAccessKeyFileEnv, "", mediaAccessKeyFileEnv + " contains an empty credential", "", " \n\t", true},
		{"secret key empty", mediaSecretKeyFileEnv, "", mediaSecretKeyFileEnv + " contains an empty credential", "", "\r\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			setValidMediaEnv(t)
			if !tt.keepEnv {
				t.Setenv(tt.env, tt.value)
			}
			if tt.fileContents != "" {
				writeMediaFile(t, os.Getenv(tt.env), tt.fileContents)
			}
			assertMediaConfigError(t, tt.want, tt.prefix)
		})
	}
}

func assertMediaConfigError(t *testing.T, want, prefix string) {
	t.Helper()
	_, err := loadMediaConfig()
	if err == nil {
		t.Fatal("load media config error is nil")
	}
	if prefix != "" {
		if got := err.Error(); !strings.HasPrefix(got, prefix) {
			t.Fatalf("error = %q, want prefix %q", got, prefix)
		}
		return
	}
	if got := err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReadMediaSecretFileNormalizesWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	writeMediaFile(t, path, "  access-key\r\n")
	got, err := readMediaSecretFile(mediaAccessKeyFileEnv, path)
	if err != nil {
		t.Fatalf("read secret: %v", err)
	}
	if got != "access-key" {
		t.Fatalf("normalized secret = %q, want %q", got, "access-key")
	}
}

func setValidMediaEnv(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	accessKeyFile, secretKeyFile := filepath.Join(dir, "access-key"), filepath.Join(dir, "secret-key")
	writeMediaFile(t, accessKeyFile, "access-key\n")
	writeMediaFile(t, secretKeyFile, "secret-key\n")
	for _, env := range []struct{ name, value string }{
		{mediaEndpointEnv, "http://rustfs:9000/"}, {mediaRegionEnv, "us-east-1"},
		{mediaBucketEnv, "sdds-media"}, {mediaPathStyleEnv, "true"},
		{mediaAccessKeyFileEnv, accessKeyFile}, {mediaSecretKeyFileEnv, secretKeyFile},
	} {
		t.Setenv(env.name, env.value)
	}
	return accessKeyFile, secretKeyFile
}

func writeMediaFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write media file: %v", err)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"SDDS_DATABASE_PATH", "SDDS_HTTP_ADDR", "SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", "SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", "SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", "SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", "SDDS_AUTH_HASH_CONCURRENCY", mediaEndpointEnv, mediaRegionEnv, mediaBucketEnv, mediaPathStyleEnv, mediaAccessKeyFileEnv, mediaSecretKeyFileEnv} {
		t.Setenv(name, "")
	}
}

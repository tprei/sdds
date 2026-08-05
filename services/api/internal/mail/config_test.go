package mail

import (
	"testing"
)

func withMailEnv(t *testing.T, fn func()) {
	t.Helper()
	for _, name := range []string{apiURLEnv, apiTokenEnv, fromAddressEnv, timeoutMsEnv} {
		t.Setenv(name, "")
	}
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(fromAddressEnv, "sender@sdds.test")
	fn()
}

func TestLoadConfigRejectsMalformedAPIURL(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "not a url", value: "not-a-url"},
		{name: "wrong scheme", value: "ftp://host/x"},
		{name: "missing scheme", value: "/relative/path"},
		{name: "embedded credentials", value: "https://user:pass@host"},
		{name: "query string", value: "https://host/?a=b"},
		{name: "fragment", value: "https://host/x#frag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withMailEnv(t, func() {
				t.Setenv(apiURLEnv, tc.value)
				if _, err := LoadConfigFromEnv(); err == nil {
					t.Fatalf("LoadConfigFromEnv with %q succeeded; want an error", tc.value)
				}
			})
		})
	}
}

func TestLoadConfigAcceptsValidAPIURLAndFallsBackToDefault(t *testing.T) {
	withMailEnv(t, func() {
		// An unset URL selects the documented default endpoint.
		config, err := LoadConfigFromEnv()
		if err != nil {
			t.Fatalf("LoadConfigFromEnv default url: %v", err)
		}
		if config.apiURL != defaultAPIURL {
			t.Fatalf("default apiURL = %q, want %q", config.apiURL, defaultAPIURL)
		}
	})

	withMailEnv(t, func() {
		t.Setenv(apiURLEnv, "https://mail-sink.local/emails")
		config, err := LoadConfigFromEnv()
		if err != nil {
			t.Fatalf("LoadConfigFromEnv valid url: %v", err)
		}
		if config.apiURL != "https://mail-sink.local/emails" {
			t.Fatalf("apiURL = %q, want https://mail-sink.local/emails", config.apiURL)
		}
	})
}

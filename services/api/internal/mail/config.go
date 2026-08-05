package mail

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	apiURLEnv        = "SDDS_MAIL_API_URL"
	apiTokenEnv      = "SDDS_MAIL_API_TOKEN"
	fromAddressEnv   = "SDDS_MAIL_FROM_ADDRESS"
	timeoutMsEnv     = "SDDS_MAIL_TIMEOUT_MS"
	modeEnv          = "SDDS_MAIL_MODE"
	modeDisabled     = "disabled"
	appBaseURLEnv    = "SDDS_APP_BASE_URL"
	defaultAPIURL    = "https://api.resend.com/emails"
	defaultTimeoutMs = 5000
)

// Config is the Resend API endpoint, credentials, and sender identity plus the
// per-call timeout. The zero value is never mistaken for a configured client:
// only LoadConfigFromEnv sets loaded, and New rejects an unloaded Config.
type Config struct {
	apiURL      string
	apiToken    string
	fromAddress string
	appBaseURL  string
	timeout     time.Duration
	loaded      bool
}

// AppBaseURL returns the public origin used to build verification and reset
// links that are embedded in delivered messages.
func (c Config) AppBaseURL() string {
	return c.appBaseURL
}

// LoadConfigFromEnv reads the transactional mail provider settings from the
// environment. SDDS_MAIL_API_URL and SDDS_MAIL_TIMEOUT_MS fall back to safe
// defaults; SDDS_MAIL_API_TOKEN and SDDS_MAIL_FROM_ADDRESS are required, since
// mail without a verified sender or credential cannot be delivered.
func LoadConfigFromEnv() (Config, error) {
	apiURL, err := absoluteHTTPURL(apiURLEnv, os.Getenv(apiURLEnv))
	if err != nil {
		return Config{}, err
	}
	apiToken, err := requiredString(apiTokenEnv)
	if err != nil {
		return Config{}, err
	}
	fromAddress, err := requiredString(fromAddressEnv)
	if err != nil {
		return Config{}, err
	}
	appBaseURL, err := requiredString(appBaseURLEnv)
	if err != nil {
		return Config{}, err
	}
	appBaseURL, err = validateAppBaseURL(appBaseURLEnv, appBaseURL)
	if err != nil {
		return Config{}, err
	}
	timeoutMs, err := positiveIntWithDefault(timeoutMsEnv, defaultTimeoutMs)
	if err != nil {
		return Config{}, err
	}
	return Config{
		apiURL:      apiURL,
		apiToken:    apiToken,
		fromAddress: fromAddress,
		appBaseURL:  appBaseURL,
		timeout:     time.Duration(timeoutMs) * time.Millisecond,
		loaded:      true,
	}, nil
}

// absoluteHTTPURL requires an absolute http(s) URL with a host and rejects
// embedded credentials, query strings, and fragments. An empty value selects
// the documented default (for SDDS_MAIL_API_URL); every other mail value is
// mandatory, so its callers use requiredString directly.
func absoluteHTTPURL(name string, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAPIURL, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	return parsed.String(), nil
}

func requiredString(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func positiveIntWithDefault(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

// DisabledByEnv reports whether transactional email is explicitly turned off
// via SDDS_MAIL_MODE=disabled. Endpoints that send mail then answer 503
// mail_unavailable instead of silently accepting requests.
func DisabledByEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(modeEnv)), modeDisabled)
}

// validateAppBaseURL requires an absolute http(s) URL and trims any trailing
// slash so verification/reset links never double-slash. http is allowed only
// for localhost so local development is not forced onto TLS.
func validateAppBaseURL(name string, raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an absolute http(s) URL without credentials, query, or fragment", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && host != "localhost" && host != "127.0.0.1" {
		return "", fmt.Errorf("%s must use https", name)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

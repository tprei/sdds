package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/tprei/sdds/services/api/internal/httpapi"
	"github.com/tprei/sdds/services/api/internal/media"
)

const (
	defaultHTTPAddr       = ":8080"
	defaultDatabasePath   = "sdds.db"
	mediaEndpointEnv      = "SDDS_MEDIA_S3_ENDPOINT"
	mediaRegionEnv        = "SDDS_MEDIA_S3_REGION"
	mediaBucketEnv        = "SDDS_MEDIA_S3_BUCKET"
	mediaPathStyleEnv     = "SDDS_MEDIA_S3_PATH_STYLE"
	mediaAccessKeyFileEnv = "SDDS_MEDIA_S3_ACCESS_KEY_FILE"
	mediaSecretKeyFileEnv = "SDDS_MEDIA_S3_SECRET_KEY_FILE"
	mediaRequestTimeout   = 15 * time.Second
	mediaRetryMaxAttempts = 3
)

type config struct {
	authLimits   httpapi.AuthLimits
	databasePath string
	httpAddr     string
	media        media.Config
}

func loadConfig() (config, error) {
	authLimits, err := loadAuthLimits()
	if err != nil {
		return config{}, err
	}

	return config{
		authLimits:   authLimits,
		databasePath: envString("SDDS_DATABASE_PATH", defaultDatabasePath),
		httpAddr:     envString("SDDS_HTTP_ADDR", defaultHTTPAddr),
	}, nil
}

func loadServerConfig() (config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return config{}, err
	}
	cfg.media, err = loadMediaConfig()
	if err != nil {
		return config{}, err
	}
	return cfg, nil
}

func loadMediaConfig() (media.Config, error) {
	endpoint, err := requiredMediaString(mediaEndpointEnv)
	if err != nil {
		return media.Config{}, err
	}
	if err := validateMediaEndpoint(endpoint); err != nil {
		return media.Config{}, err
	}
	region, err := requiredMediaString(mediaRegionEnv)
	if err != nil {
		return media.Config{}, err
	}
	bucket, err := requiredMediaString(mediaBucketEnv)
	if err != nil {
		return media.Config{}, err
	}
	usePathStyle, err := requiredMediaBool(mediaPathStyleEnv)
	if err != nil {
		return media.Config{}, err
	}
	accessKeyFile, err := requiredMediaFile(mediaAccessKeyFileEnv)
	if err != nil {
		return media.Config{}, err
	}
	secretKeyFile, err := requiredMediaFile(mediaSecretKeyFileEnv)
	if err != nil {
		return media.Config{}, err
	}

	return media.Config{
		Endpoint: endpoint, Region: region, Bucket: bucket, UsePathStyle: usePathStyle,
		AccessKeyFile: accessKeyFile, SecretKeyFile: secretKeyFile,
		Timeout: mediaRequestTimeout, RetryMaxAttempts: mediaRetryMaxAttempts,
	}, nil
}

func requiredMediaString(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return "", fmt.Errorf("%s must not contain whitespace", name)
	}
	return value, nil
}

func requiredMediaBool(name string) (bool, error) {
	switch value := strings.TrimSpace(os.Getenv(name)); value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "":
		return false, fmt.Errorf("%s is required", name)
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

func requiredMediaFile(name string) (string, error) {
	path := strings.TrimSpace(os.Getenv(name))
	if path == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if _, err := readMediaSecretFile(name, path); err != nil {
		return "", err
	}
	return path, nil
}

func readMediaSecretFile(name string, path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	secret := strings.TrimSpace(string(contents))
	if secret == "" {
		return "", fmt.Errorf("%s contains an empty credential", name)
	}
	return secret, nil
}

func validateMediaEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be a valid HTTP(S) URL", mediaEndpointEnv)
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if parsed.Hostname() == "rustfs" && parsed.Port() == "9000" && (parsed.Path == "" || parsed.Path == "/") {
			return nil
		}
		return fmt.Errorf("%s must use HTTPS except for http://rustfs:9000", mediaEndpointEnv)
	default:
		return fmt.Errorf("%s must use HTTP or HTTPS", mediaEndpointEnv)
	}
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func loadAuthLimits() (httpapi.AuthLimits, error) {
	defaults := httpapi.DefaultAuthLimits()
	signupRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE", defaults.SignupRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	loginRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE", defaults.LoginRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	signupGlobalRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE", defaults.SignupGlobalRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	loginGlobalRequestsPerMinute, err := envPositiveInt("SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE", defaults.LoginGlobalRequestsPerMinute)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}
	passwordHashConcurrency, err := envPositiveInt("SDDS_AUTH_HASH_CONCURRENCY", defaults.PasswordHashConcurrency)
	if err != nil {
		return httpapi.AuthLimits{}, err
	}

	return httpapi.AuthLimits{
		SignupRequestsPerMinute:       signupRequestsPerMinute,
		LoginRequestsPerMinute:        loginRequestsPerMinute,
		SignupGlobalRequestsPerMinute: signupGlobalRequestsPerMinute,
		LoginGlobalRequestsPerMinute:  loginGlobalRequestsPerMinute,
		PasswordHashConcurrency:       passwordHashConcurrency,
	}, nil
}

func envPositiveInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

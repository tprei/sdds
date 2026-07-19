package s3store

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	mediaEndpointEnv      = "SDDS_MEDIA_S3_ENDPOINT"
	mediaRegionEnv        = "SDDS_MEDIA_S3_REGION"
	mediaBucketEnv        = "SDDS_MEDIA_S3_BUCKET"
	mediaPathStyleEnv     = "SDDS_MEDIA_S3_PATH_STYLE"
	mediaAccessKeyFileEnv = "SDDS_MEDIA_S3_ACCESS_KEY_FILE"
	mediaSecretKeyFileEnv = "SDDS_MEDIA_S3_SECRET_KEY_FILE"
	defaultRequestTimeout = 15 * time.Second
	defaultMaxAttempts    = 3
)

type Config struct {
	endpoint         string
	region           string
	bucket           string
	usePathStyle     bool
	accessKey        string
	secretKey        string
	timeout          time.Duration
	retryMaxAttempts int
	loaded           bool
}

func LoadConfigFromEnv() (Config, error) {
	endpoint, err := requiredString(mediaEndpointEnv)
	if err != nil {
		return Config{}, err
	}
	if err := validateEndpoint(endpoint); err != nil {
		return Config{}, err
	}
	region, err := requiredString(mediaRegionEnv)
	if err != nil {
		return Config{}, err
	}
	bucket, err := requiredString(mediaBucketEnv)
	if err != nil {
		return Config{}, err
	}
	usePathStyle, err := requiredBool(mediaPathStyleEnv)
	if err != nil {
		return Config{}, err
	}
	accessKey, err := requiredCredential(mediaAccessKeyFileEnv)
	if err != nil {
		return Config{}, err
	}
	secretKey, err := requiredCredential(mediaSecretKeyFileEnv)
	if err != nil {
		return Config{}, err
	}
	return Config{
		endpoint: endpoint, region: region, bucket: bucket, usePathStyle: usePathStyle,
		accessKey: accessKey, secretKey: secretKey,
		timeout: defaultRequestTimeout, retryMaxAttempts: defaultMaxAttempts, loaded: true,
	}, nil
}

func requiredString(name string) (string, error) {
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

func requiredBool(name string) (bool, error) {
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

func requiredCredential(name string) (string, error) {
	path := strings.TrimSpace(os.Getenv(name))
	if path == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return readCredentialFile(name, path)
}

func readCredentialFile(name, path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	credential := strings.TrimSpace(string(contents))
	if credential == "" {
		return "", fmt.Errorf("%s contains an empty credential", name)
	}
	return credential, nil
}

func validateEndpoint(endpoint string) error {
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

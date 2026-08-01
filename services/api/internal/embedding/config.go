package embedding

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	urlEnv                = "SDDS_EMBEDDING_URL"
	queryTimeoutMsEnv     = "SDDS_EMBEDDING_QUERY_TIMEOUT_MS"
	batchTimeoutMsEnv     = "SDDS_EMBEDDING_BATCH_TIMEOUT_MS"
	defaultQueryTimeoutMs = 2000
	defaultBatchTimeoutMs = 30000
)

// Config is the sidecar's private-network location plus per-call timeouts.
// The zero value is never mistaken for a configured client: only
// LoadConfigFromEnv sets loaded, and New rejects an unloaded Config.
type Config struct {
	url          string
	queryTimeout time.Duration
	batchTimeout time.Duration
	loaded       bool
}

// LoadConfigFromEnv reads the embedding sidecar location from the
// environment. SDDS_EMBEDDING_URL is required -- there is no default guess,
// because a missing sidecar must fail loudly rather than silently disable
// semantic search.
func LoadConfigFromEnv() (Config, error) {
	url, err := requiredString(urlEnv)
	if err != nil {
		return Config{}, err
	}
	queryTimeoutMs, err := positiveIntWithDefault(queryTimeoutMsEnv, defaultQueryTimeoutMs)
	if err != nil {
		return Config{}, err
	}
	batchTimeoutMs, err := positiveIntWithDefault(batchTimeoutMsEnv, defaultBatchTimeoutMs)
	if err != nil {
		return Config{}, err
	}
	return Config{
		url:          url,
		queryTimeout: time.Duration(queryTimeoutMs) * time.Millisecond,
		batchTimeout: time.Duration(batchTimeoutMs) * time.Millisecond,
		loaded:       true,
	}, nil
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

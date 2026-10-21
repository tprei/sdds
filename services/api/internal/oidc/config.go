package oidc

import (
	"fmt"
	"os"
	"strings"
)

type providerRecord struct {
	issuers   []string
	jwksURL   string
	audiences []string
}

type Config struct {
	providers map[Provider]providerRecord
}

const (
	modeEnv            = "SDDS_AUTH_OIDC_MODE"
	modeDisabled       = "disabled"
	modeEnabled        = "enabled"
	googleAudiencesEnv = "SDDS_AUTH_OIDC_GOOGLE_AUDIENCES"
	appleAudiencesEnv  = "SDDS_AUTH_OIDC_APPLE_AUDIENCES"
)

// LoadFromEnv returns a nil client when provider sign-in is turned off.
func LoadFromEnv() (*Client, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(modeEnv)))
	if mode == "" || mode == modeDisabled {
		return nil, nil
	}
	if mode != modeEnabled {
		return nil, fmt.Errorf("%s must be %q or %q", modeEnv, modeEnabled, modeDisabled)
	}

	providers := make(map[Provider]providerRecord, 2)
	googleAudiences, err := parseAudiences(googleAudiencesEnv)
	if err != nil {
		return nil, err
	}
	if len(googleAudiences) > 0 {
		providers[ProviderGoogle] = providerRecord{
			issuers:   append([]string(nil), providerIssuers[ProviderGoogle]...),
			jwksURL:   googleJWKSURL,
			audiences: googleAudiences,
		}
	}

	appleAudiences, err := parseAudiences(appleAudiencesEnv)
	if err != nil {
		return nil, err
	}
	if len(appleAudiences) > 0 {
		providers[ProviderApple] = providerRecord{
			issuers:   append([]string(nil), providerIssuers[ProviderApple]...),
			jwksURL:   appleJWKSURL,
			audiences: appleAudiences,
		}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("%s=enabled requires at least one provider audience list", modeEnv)
	}
	return newClient(Config{providers: providers}), nil
}

func parseAudiences(name string) ([]string, error) {
	raw, present := os.LookupEnv(name)
	if !present || raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	audiences := make([]string, 0, len(parts))
	for _, part := range parts {
		audience := strings.TrimSpace(part)
		if audience == "" {
			return nil, fmt.Errorf("%s must be a comma-separated list of audiences", name)
		}
		audiences = append(audiences, audience)
	}
	if len(audiences) == 0 {
		return nil, fmt.Errorf("%s must be a comma-separated list of audiences", name)
	}
	return audiences, nil
}

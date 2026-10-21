package oidc

import (
	"context"
	"errors"
)

type Provider string

const (
	ProviderApple  Provider = "apple"
	ProviderGoogle Provider = "google"
)

var (
	ErrUnavailable  = errors.New("oidc verification unavailable")
	ErrInvalidToken = errors.New("oidc token invalid")
)

// Identity is the verified subset of an id_token this service trusts.
type Identity struct {
	Provider      Provider
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type Verifier interface {
	Verify(ctx context.Context, provider Provider, idToken string, nonce string) (Identity, error)
}

const (
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	appleJWKSURL  = "https://appleid.apple.com/auth/keys"
)

var providerIssuers = map[Provider][]string{
	ProviderGoogle: {"https://accounts.google.com", "accounts.google.com"},
	ProviderApple:  {"https://appleid.apple.com"},
}

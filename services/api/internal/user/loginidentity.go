package user

import (
	"context"
	"errors"
	"time"
)

const (
	LoginIdentityProviderApple  = "apple"
	LoginIdentityProviderGoogle = "google"
)

const (
	ContactChannelVerifiedViaApple  = "apple"
	ContactChannelVerifiedViaGoogle = "google"
)

var (
	ErrUsernameRequired      = errors.New("username required")
	ErrLoginIdentityNotFound = errors.New("login identity not found")
	ErrLastLoginIdentity     = errors.New("last login identity")
)

// emailNamespaceOwningProviders lists the IdPs that own the address namespace
// they assert. Linking an account on a provider email is safe only for these;
// a provider that lets a user self-assert an arbitrary address would turn the
// linking rule into account takeover.
var emailNamespaceOwningProviders = map[string]struct{}{
	LoginIdentityProviderApple:  {},
	LoginIdentityProviderGoogle: {},
}

func ProviderOwnsEmailNamespace(provider string) bool {
	_, ok := emailNamespaceOwningProviders[provider]
	return ok
}

type ResolveOIDCIdentityInput struct {
	Provider      string
	Subject       string
	Email         string // normalized; empty when absent
	EmailVerified bool
	DisplayName   string // empty when the provider gave none
	Username      string // empty until the client supplies one
	TokenHash     string
	ExpiresAt     time.Time
}

type LoginIdentitySummary struct {
	ID       LoginIdentityID
	Kind     LoginIdentityKind
	Provider string
}

type LoginIdentityStore interface {
	ResolveOIDCIdentity(ctx context.Context, input ResolveOIDCIdentityInput) (CurrentSession, error)
	ListLoginIdentities(ctx context.Context, userID UserID) ([]LoginIdentitySummary, error)
	DeleteLoginIdentity(ctx context.Context, userID UserID, identityID LoginIdentityID) error
}

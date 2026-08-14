package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func verifyClaims(issuer *fakeIssuer, mutate func(map[string]any)) string {
	claims := issuer.validClaims()
	if mutate != nil {
		mutate(claims)
	}
	return issuer.mint(claims)
}

func requireInvalidToken(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifierValidTokenPerProvider(t *testing.T) {
	for _, provider := range []Provider{ProviderGoogle, ProviderApple} {
		t.Run(string(provider), func(t *testing.T) {
			issuer := newFakeIssuer(t, provider)
			identity, err := issuer.client().Verify(context.Background(), provider, verifyClaims(issuer, nil), "request-nonce")
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			want := Identity{
				Provider:      provider,
				Subject:       "subject-1",
				Email:         "person@example.com",
				EmailVerified: true,
				DisplayName:   "Person Example",
			}
			if identity != want {
				t.Fatalf("Identity = %#v, want %#v", identity, want)
			}
		})
	}
}

func TestVerifierAcceptsGoogleBareIssuer(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	client := issuer.client()
	client.providers[ProviderGoogle] = providerRecord{
		issuers:   []string{issuer.issuer, "accounts.google.com"},
		jwksURL:   issuer.server.URL,
		audiences: []string{issuer.audience},
	}
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["iss"] = "accounts.google.com"
	})
	if _, err := client.Verify(context.Background(), ProviderGoogle, token, "request-nonce"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifierAcceptsAudienceArrayMembership(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["aud"] = []string{"other-client", issuer.audience}
	})
	if _, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifierRejectsForgedSignature(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	forgedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	token := issuer.mintWith(issuer.keys[0], jwt.SigningMethodRS256, issuer.validClaims(), forgedKey)
	_, err = issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsUnknownKid(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	unknown := issuer.generateKey("unknown-kid")
	token := issuer.mintWith(unknown, jwt.SigningMethodRS256, issuer.validClaims(), unknown.private)
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsAlgNone(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := issuer.mintWith(issuer.keys[0], jwt.SigningMethodNone, issuer.validClaims(), jwt.UnsafeAllowNoneSignatureType)
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsHS256(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := issuer.mintWith(issuer.keys[0], jwt.SigningMethodHS256, issuer.validClaims(), []byte("shared-secret"))
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsWrongIssuer(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["iss"] = "https://attacker.example"
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsWrongAudienceWithValidSignature(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["aud"] = "different-client"
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsExpiredToken(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["exp"] = time.Now().Add(-2 * time.Minute).Unix()
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRequiresExpiration(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		delete(claims, "exp")
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsMissingNonce(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		delete(claims, "nonce")
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsEmptyNonce(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["nonce"] = ""
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsWrongNonce(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["nonce"] = "different-nonce"
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsEmptySuppliedNonce(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	emptyDigest := sha256.Sum256(nil)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["nonce"] = hex.EncodeToString(emptyDigest[:])
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "")
	requireInvalidToken(t, err)
}
func TestVerifierAcceptsHashedNonce(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderApple)
	digest := sha256.Sum256([]byte("request-nonce"))
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["nonce"] = hex.EncodeToString(digest[:])
	})
	if _, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifierEmailVerifiedCoercion(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "true", value: true, want: true},
		{name: "false", value: false, want: false},
		{name: "true string", value: "true", want: true},
		{name: "uppercase true string", value: "TRUE", want: true},
		{name: "false string", value: "false", want: false},
		{name: "one", value: float64(1), want: true},
		{name: "zero", value: float64(0), want: false},
		{name: "absent", want: false},
		{name: "null", value: nil, want: false},
		{name: "yes string", value: "yes", want: false},
		{name: "array", value: []any{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issuer := newFakeIssuer(t, ProviderGoogle)
			token := verifyClaims(issuer, func(claims map[string]any) {
				if test.name != "absent" {
					claims["email_verified"] = test.value
				} else {
					delete(claims, "email_verified")
				}
			})
			identity, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if identity.EmailVerified != test.want {
				t.Fatalf("EmailVerified = %v, want %v", identity.EmailVerified, test.want)
			}
		})
	}
}

func TestVerifierCoerceEmailVerifiedJSONNumber(t *testing.T) {
	if !coerceEmailVerified(json.Number("1")) {
		t.Fatal("coerceEmailVerified(json.Number(\"1\")) = false")
	}
	if coerceEmailVerified(json.Number("1.0")) {
		t.Fatal("coerceEmailVerified(json.Number(\"1.0\")) = true")
	}
}

func TestVerifierRejectsMissingSubject(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		delete(claims, "sub")
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierRejectsEmptySubject(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	token := verifyClaims(issuer, func(claims map[string]any) {
		claims["sub"] = ""
	})
	_, err := issuer.client().Verify(context.Background(), issuer.provider, token, "request-nonce")
	requireInvalidToken(t, err)
}

func TestVerifierReturnsUnavailableForJWKS500(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderApple)
	issuer.setStatus(500)
	_, err := issuer.client().Verify(context.Background(), issuer.provider, verifyClaims(issuer, nil), "request-nonce")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() error = %v, want ErrUnavailable", err)
	}
}

func TestVerifierReturnsUnavailableForUnknownProvider(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	_, err := issuer.client().Verify(context.Background(), ProviderApple, verifyClaims(issuer, nil), "request-nonce")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() error = %v, want ErrUnavailable", err)
	}
}

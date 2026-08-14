package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestJWKSInitialFetchAndKeyLookup(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	client := issuer.client()

	key, err := client.key(context.Background(), issuer.provider, issuer.keys[0].kid)
	if err != nil {
		t.Fatalf("key() error = %v", err)
	}
	if key == nil || key.N.Cmp(issuer.keys[0].private.N) != 0 {
		t.Fatal("key() returned the wrong RSA key")
	}
	if got := issuer.hits.Load(); got != 1 {
		t.Fatalf("JWKS hits = %d, want 1", got)
	}
}

func TestJWKSRotationRefetchesUnknownKid(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	client := issuer.client()
	oldKey := issuer.keys[0]
	if _, err := client.key(context.Background(), issuer.provider, oldKey.kid); err != nil {
		t.Fatalf("initial key() error = %v", err)
	}

	rotatedKey := issuer.generateKey("kid-2")
	issuer.setKeys(rotatedKey)
	key, err := client.key(context.Background(), issuer.provider, rotatedKey.kid)
	if err != nil {
		t.Fatalf("rotated key() error = %v", err)
	}
	if key.N.Cmp(rotatedKey.private.N) != 0 {
		t.Fatal("rotation returned the wrong key")
	}
	if got := issuer.hits.Load(); got != 2 {
		t.Fatalf("JWKS hits = %d, want 2", got)
	}
}

func TestJWKSUnknownKidRefetchIsRateLimited(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	client := issuer.client()
	if _, err := client.key(context.Background(), issuer.provider, issuer.keys[0].kid); err != nil {
		t.Fatalf("initial key() error = %v", err)
	}

	for _, kid := range []string{"garbage-1", "garbage-2"} {
		if _, err := client.key(context.Background(), issuer.provider, kid); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("key(%q) error = %v, want ErrInvalidToken", kid, err)
		}
	}
	if got := issuer.hits.Load(); got != 2 {
		t.Fatalf("JWKS hits = %d, want one forced refetch", got)
	}
}

func TestJWKSTTLExpiryRefetchesKnownKid(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	client := issuer.client()
	now := time.Now()
	client.clock = func() time.Time { return now }

	kid := issuer.keys[0].kid
	if _, err := client.key(context.Background(), issuer.provider, kid); err != nil {
		t.Fatalf("initial key() error = %v", err)
	}
	now = now.Add(jwksTTL)
	if _, err := client.key(context.Background(), issuer.provider, kid); err != nil {
		t.Fatalf("expired-cache key() error = %v", err)
	}
	if got := issuer.hits.Load(); got != 2 {
		t.Fatalf("JWKS hits = %d, want 2 after TTL expiry", got)
	}
}

func TestJWKSFetchFailureReturnsUnavailable(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	issuer.setStatus(500)
	client := issuer.client()

	_, err := client.key(context.Background(), issuer.provider, issuer.keys[0].kid)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("key() error = %v, want ErrUnavailable", err)
	}
}

func TestJWKSTransportFailureReturnsUnavailable(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	kid := issuer.keys[0].kid
	issuer.server.Close()
	client := issuer.client()

	_, err := client.key(context.Background(), issuer.provider, kid)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("key() error = %v, want ErrUnavailable", err)
	}
}

func TestJWKSRejectsEmptyKeySetAsUnavailable(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	kid := issuer.keys[0].kid
	issuer.setKeys()
	client := issuer.client()

	_, err := client.key(context.Background(), issuer.provider, kid)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("key() error = %v, want ErrUnavailable", err)
	}
}

func TestJWKSParsesPaddedRSAKeyAndSkipsUnsupportedKeys(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	document := jwksDocument{Keys: []jwk{
		{KTY: "EC", Kid: "ec", Alg: "RS256", N: "bad", E: "bad"},
		{
			KTY: "RSA",
			Kid: "rsa",
			Alg: "RS256",
			N:   base64.URLEncoding.EncodeToString(private.N.Bytes()),
			E:   base64.URLEncoding.EncodeToString(bigEndianExponent(private.E)),
		},
	}}
	keys, err := parseJWKS(document)
	if err != nil {
		t.Fatalf("parseJWKS() error = %v", err)
	}
	if len(keys) != 1 || keys["rsa"].N.Cmp(private.N) != 0 || keys["rsa"].E != private.E {
		t.Fatalf("parsed keys = %#v", keys)
	}
}

func TestJWKSRejectsGarbageBodyAsUnavailable(t *testing.T) {
	issuer := newFakeIssuer(t, ProviderGoogle)
	issuer.keysMu.Lock()
	issuer.jwks = []byte("not-json")
	issuer.keysMu.Unlock()
	client := issuer.client()

	_, err := client.key(context.Background(), issuer.provider, issuer.keys[0].kid)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("key() error = %v, want ErrUnavailable", err)
	}
}

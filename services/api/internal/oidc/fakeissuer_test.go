package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeIssuerKey struct {
	kid     string
	private *rsa.PrivateKey
	alg     string
}

type fakeIssuer struct {
	t        *testing.T
	provider Provider
	issuer   string
	audience string
	server   *httptest.Server
	keysMu   sync.RWMutex
	keys     []fakeIssuerKey
	status   int
	jwks     []byte
	hits     atomic.Int64
}

func newFakeIssuer(t *testing.T, provider Provider) *fakeIssuer {
	t.Helper()
	issuer := &fakeIssuer{
		t:        t,
		provider: provider,
		issuer:   "https://fake.example/" + string(provider),
		audience: "fake-client-" + string(provider),
		status:   http.StatusOK,
	}
	issuer.keys = []fakeIssuerKey{issuer.generateKey("kid-1")}
	issuer.server = httptest.NewServer(http.HandlerFunc(issuer.serveJWKS))
	issuer.setJWKS()
	t.Cleanup(issuer.server.Close)
	return issuer
}

func (f *fakeIssuer) generateKey(kid string) fakeIssuerKey {
	f.t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.t.Fatalf("generate RSA key: %v", err)
	}
	return fakeIssuerKey{kid: kid, private: private, alg: "RS256"}
}

func (f *fakeIssuer) serveJWKS(w http.ResponseWriter, _ *http.Request) {
	f.hits.Add(1)
	f.keysMu.RLock()
	status := f.status
	body := append([]byte(nil), f.jwks...)
	f.keysMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (f *fakeIssuer) setStatus(status int) {
	f.keysMu.Lock()
	f.status = status
	f.keysMu.Unlock()
}

func (f *fakeIssuer) setKeys(keys ...fakeIssuerKey) {
	f.keysMu.Lock()
	f.keys = append([]fakeIssuerKey(nil), keys...)
	f.setJWKSLocked()
	f.keysMu.Unlock()
}

func (f *fakeIssuer) setJWKS() {
	f.keysMu.Lock()
	f.setJWKSLocked()
	f.keysMu.Unlock()
}

func (f *fakeIssuer) setJWKSLocked() {
	document := map[string]any{"keys": make([]map[string]string, 0, len(f.keys))}
	for _, key := range f.keys {
		document["keys"] = append(document["keys"].([]map[string]string), map[string]string{
			"kty": "RSA",
			"kid": key.kid,
			"alg": key.alg,
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.private.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(bigEndianExponent(key.private.E)),
		})
	}
	body, err := json.Marshal(document)
	if err != nil {
		f.t.Fatalf("marshal JWKS: %v", err)
	}
	f.jwks = body
}

func bigEndianExponent(exponent int) []byte {
	if exponent == 0 {
		return []byte{0}
	}
	result := make([]byte, 0, 4)
	for exponent > 0 {
		result = append([]byte{byte(exponent)}, result...)
		exponent >>= 8
	}
	return result
}

func (f *fakeIssuer) config() Config {
	return Config{providers: map[Provider]providerRecord{
		f.provider: {
			issuers:   []string{f.issuer},
			jwksURL:   f.server.URL,
			audiences: []string{f.audience},
		},
	}}
}

func (f *fakeIssuer) client() *Client {
	return newClient(f.config())
}

func (f *fakeIssuer) validClaims() map[string]any {
	return map[string]any{
		"iss":            f.issuer,
		"aud":            f.audience,
		"nonce":          "request-nonce",
		"sub":            "subject-1",
		"email":          " Person@Example.COM ",
		"email_verified": true,
		"name":           "  Person Example  ",
		"exp":            time.Now().Add(time.Hour).Unix(),
	}
}

func (f *fakeIssuer) mint(claims map[string]any) string {
	f.t.Helper()
	f.keysMu.RLock()
	key := f.keys[0]
	f.keysMu.RUnlock()
	return f.mintWith(key, jwt.SigningMethodRS256, claims, key.private)
}

func (f *fakeIssuer) mintWith(key fakeIssuerKey, method jwt.SigningMethod, claims map[string]any, signingKey any) string {
	f.t.Helper()
	copyClaims := make(map[string]any, len(claims))
	for name, value := range claims {
		copyClaims[name] = value
	}
	token := jwt.NewWithClaims(method, jwt.MapClaims(copyClaims))
	token.Header["kid"] = key.kid
	encoded, err := token.SignedString(signingKey)
	if err != nil {
		f.t.Fatalf("sign token: %v", err)
	}
	return encoded
}

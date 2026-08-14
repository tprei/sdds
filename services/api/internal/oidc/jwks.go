package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"crypto/rsa"

	"golang.org/x/time/rate"
)

const (
	jwksTTL          = time.Hour
	jwksFetchTimeout = 5 * time.Second
	jwksMaxBodyBytes = 1 << 20
	clockSkewLeeway  = time.Minute
)

type Client struct {
	httpClient *http.Client
	clock      func() time.Time
	providers  map[Provider]providerRecord
	keys       map[Provider]*keyCache
}

type keyCache struct {
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	refetch   *rate.Limiter
}

func newClient(config Config) *Client {
	providers := make(map[Provider]providerRecord, len(config.providers))
	keys := make(map[Provider]*keyCache, len(config.providers))
	for provider, record := range config.providers {
		providers[provider] = providerRecord{
			issuers:   append([]string(nil), record.issuers...),
			jwksURL:   record.jwksURL,
			audiences: append([]string(nil), record.audiences...),
		}
		keys[provider] = &keyCache{
			keys:    make(map[string]*rsa.PublicKey),
			refetch: rate.NewLimiter(rate.Every(time.Minute), 1),
		}
	}
	return &Client{
		httpClient: &http.Client{},
		clock:      time.Now,
		providers:  providers,
		keys:       keys,
	}
}

func (c *Client) key(ctx context.Context, provider Provider, kid string) (*rsa.PublicKey, error) {
	record, ok := c.providers[provider]
	if !ok {
		return nil, ErrUnavailable
	}
	cache, ok := c.keys[provider]
	if !ok || cache == nil {
		return nil, ErrUnavailable
	}

	now := c.now()
	cache.mu.Lock()
	if cache.keys == nil {
		cache.keys = make(map[string]*rsa.PublicKey)
	}
	if cache.refetch == nil {
		cache.refetch = rate.NewLimiter(rate.Every(time.Minute), 1)
	}
	defer cache.mu.Unlock()

	if key, ok := cache.keys[kid]; ok && now.Sub(cache.fetchedAt) < jwksTTL {
		return key, nil
	}
	// A refetch is unconditional only for an empty cache, which has nothing to
	// serve. Once keys exist, every refetch — TTL expiry or an unknown kid —
	// costs a limiter token, so an upstream outage or a stream of garbage kids
	// cannot amplify into a matching stream of upstream requests.
	if len(cache.keys) > 0 && !cache.refetch.AllowN(now, 1) {
		if key, ok := cache.keys[kid]; ok {
			return key, nil
		}
		return nil, ErrInvalidToken
	}

	keys, err := c.fetch(ctx, record)
	if err != nil {
		return nil, err
	}
	cache.keys = keys
	cache.fetchedAt = c.now()
	key, ok := cache.keys[kid]
	if !ok {
		return nil, ErrInvalidToken
	}
	return key, nil
}

func (c *Client) fetch(ctx context.Context, record providerRecord) (map[string]*rsa.PublicKey, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, jwksFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, record.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build JWKS request: %v", ErrUnavailable, err)
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: fetch JWKS: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%w: JWKS status %d", ErrUnavailable, resp.StatusCode)
	}

	var document jwksDocument
	decoder := json.NewDecoder(io.LimitReader(resp.Body, jwksMaxBodyBytes))
	if err := decoder.Decode(&document); err != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%w: decode JWKS: %v", ErrUnavailable, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		_, _ = io.Copy(io.Discard, resp.Body)
		if err == nil {
			return nil, fmt.Errorf("%w: decode JWKS: multiple JSON values", ErrUnavailable)
		}
		return nil, fmt.Errorf("%w: decode JWKS: %v", ErrUnavailable, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)

	keys, err := parseJWKS(document)
	if err != nil {
		return nil, fmt.Errorf("%w: parse JWKS: %v", ErrUnavailable, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: JWKS has no usable keys", ErrUnavailable)
	}
	return keys, nil
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KTY string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func parseJWKS(document jwksDocument) (map[string]*rsa.PublicKey, error) {
	keys := make(map[string]*rsa.PublicKey)
	for _, key := range document.Keys {
		if key.KTY != "RSA" || (key.Alg != "" && key.Alg != "RS256") {
			continue
		}
		if key.Kid == "" || key.N == "" || key.E == "" {
			return nil, errors.New("RSA key is missing a required field")
		}
		modulus, err := decodeBase64URL(key.N)
		if err != nil {
			return nil, fmt.Errorf("decode RSA modulus: %v", err)
		}
		exponent, err := decodeBase64URL(key.E)
		if err != nil {
			return nil, fmt.Errorf("decode RSA exponent: %v", err)
		}
		modulusInt := new(big.Int).SetBytes(modulus)
		if modulusInt.BitLen() < 2048 {
			return nil, errors.New("RSA modulus is smaller than 2048 bits")
		}
		exponentInt := new(big.Int).SetBytes(exponent)
		if !exponentInt.IsInt64() || exponentInt.Sign() <= 0 {
			return nil, errors.New("RSA exponent is out of range")
		}
		exponentValue := exponentInt.Int64()
		if exponentValue > int64(^uint(0)>>1) {
			return nil, errors.New("RSA exponent is out of range")
		}
		if exponentValue < 3 || exponentValue%2 == 0 {
			return nil, errors.New("RSA exponent is not a usable public exponent")
		}
		keys[key.Kid] = &rsa.PublicKey{
			N: modulusInt,
			E: int(exponentValue),
		}
	}
	return keys, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}

func (c *Client) now() time.Time {
	if c.clock == nil {
		return time.Now()
	}
	return c.clock()
}

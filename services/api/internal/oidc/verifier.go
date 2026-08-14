package oidc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func (c *Client) Verify(ctx context.Context, provider Provider, idToken string, nonce string) (Identity, error) {
	record, ok := c.providers[provider]
	if !ok {
		return Identity{}, ErrUnavailable
	}
	if nonce == "" {
		return Identity{}, ErrInvalidToken
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockSkewLeeway),
		jwt.WithTimeFunc(c.now),
	)
	claims := jwt.MapClaims{}
	token, err := parser.ParseWithClaims(idToken, claims, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, ErrInvalidToken
		}
		return c.key(ctx, provider, kid)
	})
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return Identity{}, ErrUnavailable
		}
		return Identity{}, ErrInvalidToken
	}
	if token == nil || !token.Valid {
		return Identity{}, ErrInvalidToken
	}

	issuer, ok := claims["iss"].(string)
	if !ok || !contains(record.issuers, issuer) {
		return Identity{}, ErrInvalidToken
	}
	if !audienceMatches(claims["aud"], record.audiences) {
		return Identity{}, ErrInvalidToken
	}
	nonceClaim, ok := claims["nonce"].(string)
	if !ok || nonceClaim == "" || !nonceMatches(nonceClaim, nonce) {
		return Identity{}, ErrInvalidToken
	}
	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return Identity{}, ErrInvalidToken
	}

	email, _ := claims["email"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	name, _ := claims["name"].(string)

	return Identity{
		Provider:      provider,
		Subject:       subject,
		Email:         email,
		EmailVerified: coerceEmailVerified(claims["email_verified"]),
		DisplayName:   strings.TrimSpace(name),
	}, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func audienceMatches(value any, audiences []string) bool {
	if len(audiences) == 0 {
		return false
	}
	switch typed := value.(type) {
	case string:
		return contains(audiences, typed)
	case []string:
		for _, audience := range typed {
			if contains(audiences, audience) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			audience, ok := item.(string)
			if ok && contains(audiences, audience) {
				return true
			}
		}
	}
	return false
}

func nonceMatches(claim string, nonce string) bool {
	rawMatch := subtle.ConstantTimeCompare([]byte(claim), []byte(nonce))
	digest := sha256.Sum256([]byte(nonce))
	hashedNonce := hex.EncodeToString(digest[:])
	hashedMatch := subtle.ConstantTimeCompare([]byte(claim), []byte(hashedNonce))
	return rawMatch == 1 || hashedMatch == 1
}

func coerceEmailVerified(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	case float64:
		return typed == 1
	case json.Number:
		return typed.String() == "1"
	default:
		return false
	}
}

var _ Verifier = (*Client)(nil)

package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

const SessionLifetime = 30 * 24 * time.Hour

func NewSessionToken() (string, error) {
	return newRandomToken()
}

func HashSessionToken(token string) string {
	return hashToken(token)
}

// newRandomToken returns 32 random bytes as an unpadded base64url string.
func newRandomToken() (string, error) {
	token := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, token); err != nil {
		return "", fmt.Errorf("read session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// hashToken returns the lowercase hex sha256 of a token string.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

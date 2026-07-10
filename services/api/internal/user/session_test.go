package user

import "testing"

func TestNewSessionTokenGeneratesBase64URLToken(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("create session token: %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("token length = %d, want at least 40", len(token))
	}
	for _, value := range token {
		if value == '+' || value == '/' || value == '=' {
			t.Fatalf("token contains non-url-safe character %q", value)
		}
	}
}

func TestHashSessionTokenIsStableAndTokenSpecific(t *testing.T) {
	first := HashSessionToken("token-a")
	if first != HashSessionToken("token-a") {
		t.Fatal("same token hash changed")
	}
	if first == HashSessionToken("token-b") {
		t.Fatal("different tokens produced same hash")
	}
	if len(first) != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars", len(first))
	}
}

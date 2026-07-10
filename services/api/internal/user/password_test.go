package user

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestPasswordHasherVerifiesPassword(t *testing.T) {
	hasher := testHasherWithSalt(1)
	hash, err := hasher.Hash("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	verified, err := hasher.Verify("secret-password", hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !verified {
		t.Fatal("verified = false, want true")
	}

	verified, err = hasher.Verify("wrong-password", hash)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if verified {
		t.Fatal("verified wrong password = true, want false")
	}
}

func TestPasswordHasherRejectsMalformedHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{name: "not argon2", hash: "not-an-argon2-hash"},
		{name: "zero iterations", hash: "argon2id$v=19$m=1024,t=0,p=1$" + encodedBytes(16, 1) + "$" + encodedBytes(32, 2)},
		{name: "zero parallelism", hash: "argon2id$v=19$m=1024,t=1,p=0$" + encodedBytes(16, 1) + "$" + encodedBytes(32, 2)},
		{name: "huge memory", hash: "argon2id$v=19$m=1048576,t=1,p=1$" + encodedBytes(16, 1) + "$" + encodedBytes(32, 2)},
		{name: "short salt", hash: "argon2id$v=19$m=1024,t=1,p=1$" + encodedBytes(4, 1) + "$" + encodedBytes(32, 2)},
		{name: "short key", hash: "argon2id$v=19$m=1024,t=1,p=1$" + encodedBytes(16, 1) + "$" + encodedBytes(4, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestPasswordHasher().Verify("secret-password", tt.hash)
			if !errors.Is(err, ErrPasswordHashMalformed) {
				t.Fatalf("verify error = %v, want ErrPasswordHashMalformed", err)
			}
		})
	}
}

func TestPasswordHasherUsesDifferentSalts(t *testing.T) {
	first, err := testHasherWithSalt(1).Hash("secret-password")
	if err != nil {
		t.Fatalf("hash first password: %v", err)
	}
	second, err := testHasherWithSalt(2).Hash("secret-password")
	if err != nil {
		t.Fatalf("hash second password: %v", err)
	}
	if first == second {
		t.Fatal("hashes match, want different salt output")
	}
}

func TestPasswordHasherProductionHashParsesAndVerifies(t *testing.T) {
	hasher := NewPasswordHasher()
	hasher.SaltReader = bytes.NewReader(bytes.Repeat([]byte{7}, int(hasher.Params.SaltLength)))
	hash, err := hasher.Hash("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, "argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("hash prefix = %q, want production params", hash)
	}

	verified, err := hasher.Verify("secret-password", hash)
	if err != nil {
		t.Fatalf("verify production hash: %v", err)
	}
	if !verified {
		t.Fatal("verified = false, want true")
	}
}

func testHasherWithSalt(value byte) PasswordHasher {
	hasher := newTestPasswordHasher()
	hasher.SaltReader = bytes.NewReader(bytes.Repeat([]byte{value}, int(hasher.Params.SaltLength)))
	return hasher
}

func encodedBytes(length int, value byte) string {
	return base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{value}, length))
}

func newTestPasswordHasher() PasswordHasher {
	return PasswordHasher{
		Params: Argon2idParams{
			Memory:      1024,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		},
		SaltReader: nil,
	}
}

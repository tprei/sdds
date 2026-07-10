package user

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

var ErrPasswordHashMalformed = errors.New("password hash malformed")

type Argon2idParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

const (
	passwordHashMinMemory      uint32 = 1024
	passwordHashMaxMemory      uint32 = 128 * 1024
	passwordHashMinIterations  uint32 = 1
	passwordHashMaxIterations  uint32 = 10
	passwordHashMinParallelism uint8  = 1
	passwordHashMaxParallelism uint8  = 4
	passwordHashSaltLength     uint32 = 16
	passwordHashKeyLength      uint32 = 32
)

type PasswordHasher struct {
	Params     Argon2idParams
	SaltReader io.Reader
}

func NewPasswordHasher() PasswordHasher {
	return PasswordHasher{
		Params: Argon2idParams{
			Memory:      64 * 1024,
			Iterations:  3,
			Parallelism: 1,
			SaltLength:  passwordHashSaltLength,
			KeyLength:   passwordHashKeyLength,
		},
		SaltReader: rand.Reader,
	}
}

func (hasher PasswordHasher) Hash(password string) (string, error) {
	params := hasher.Params
	saltReader := hasher.SaltReader
	if saltReader == nil {
		saltReader = rand.Reader
	}

	salt := make([]byte, params.SaltLength)
	if _, err := io.ReadFull(saltReader, salt); err != nil {
		return "", fmt.Errorf("read password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	return encodePasswordHash(params, salt, hash), nil
}

func (hasher PasswordHasher) Verify(password string, encoded string) (bool, error) {
	params, salt, wantHash, err := decodePasswordHash(encoded)
	if err != nil {
		return false, err
	}

	gotHash := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	return subtle.ConstantTimeCompare(gotHash, wantHash) == 1, nil
}

func encodePasswordHash(params Argon2idParams, salt []byte, hash []byte) string {
	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func decodePasswordHash(encoded string) (Argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return Argon2idParams{}, nil, nil, ErrPasswordHashMalformed
	}

	params, err := decodePasswordHashParams(parts[2])
	if err != nil {
		return Argon2idParams{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return Argon2idParams{}, nil, nil, ErrPasswordHashMalformed
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Argon2idParams{}, nil, nil, ErrPasswordHashMalformed
	}
	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(hash))
	if !validPasswordHashParams(params) {
		return Argon2idParams{}, nil, nil, ErrPasswordHashMalformed
	}
	return params, salt, hash, nil
}

func decodePasswordHashParams(encoded string) (Argon2idParams, error) {
	parts := strings.Split(encoded, ",")
	if len(parts) != 3 {
		return Argon2idParams{}, ErrPasswordHashMalformed
	}

	memory, err := parseUint32Param(parts[0], "m")
	if err != nil {
		return Argon2idParams{}, err
	}
	iterations, err := parseUint32Param(parts[1], "t")
	if err != nil {
		return Argon2idParams{}, err
	}
	parallelism, err := parseUint8Param(parts[2], "p")
	if err != nil {
		return Argon2idParams{}, err
	}

	return Argon2idParams{
		Memory:      memory,
		Iterations:  iterations,
		Parallelism: parallelism,
	}, nil
}

func validPasswordHashParams(params Argon2idParams) bool {
	return params.Memory >= passwordHashMinMemory &&
		params.Memory <= passwordHashMaxMemory &&
		params.Iterations >= passwordHashMinIterations &&
		params.Iterations <= passwordHashMaxIterations &&
		params.Parallelism >= passwordHashMinParallelism &&
		params.Parallelism <= passwordHashMaxParallelism &&
		params.SaltLength == passwordHashSaltLength &&
		params.KeyLength == passwordHashKeyLength
}

func parseUint32Param(part string, key string) (uint32, error) {
	value, ok := strings.CutPrefix(part, key+"=")
	if !ok {
		return 0, ErrPasswordHashMalformed
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, ErrPasswordHashMalformed
	}
	return uint32(parsed), nil
}

func parseUint8Param(part string, key string) (uint8, error) {
	value, ok := strings.CutPrefix(part, key+"=")
	if !ok {
		return 0, ErrPasswordHashMalformed
	}
	parsed, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return 0, ErrPasswordHashMalformed
	}
	return uint8(parsed), nil
}

package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

const MaxCursorLength = 160

var ErrCursorTooLong = errors.New("cursor exceeds maximum length")

var ErrInvalidCursor = errors.New("invalid cursor")

func Encode(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	if len(encoded) > MaxCursorLength {
		return "", ErrCursorTooLong
	}
	return encoded, nil
}

func Decode(encoded string, target any) error {
	if encoded == "" || len(encoded) > MaxCursorLength {
		return ErrInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ErrInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidCursor
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidCursor
	}
	return nil
}

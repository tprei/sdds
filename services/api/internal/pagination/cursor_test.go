package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

type testCursor struct {
	Version   int    `json:"v"`
	CreatedAt int64  `json:"created_at"`
	ID        string `json:"id"`
}

func TestCursorRoundTrip(t *testing.T) {
	want := testCursor{Version: 1, CreatedAt: 1782993600000, ID: "opaque-note-id"}
	encoded, err := Encode(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got testCursor
	if err := Decode(encoded, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != want {
		t.Fatalf("decoded cursor = %#v, want %#v", got, want)
	}
}

func TestCursorRejectsMalformedPayloads(t *testing.T) {
	valid := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"created_at":1782993600000,"id":"opaque"}`))
	tests := []struct {
		name    string
		encoded string
	}{
		{name: "empty", encoded: ""},
		{name: "invalid base64", encoded: "not-base64!"},
		{name: "unknown field", encoded: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"created_at":1782993600000,"id":"opaque","extra":true}`))},
		{name: "trailing json", encoded: valid + base64.RawURLEncoding.EncodeToString([]byte(`{"v":1}`))},
		{name: "oversized", encoded: strings.Repeat("a", MaxCursorLength+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target testCursor
			if err := Decode(tt.encoded, &target); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("decode error = %v, want ErrInvalidCursor", err)
			}
		})
	}
}

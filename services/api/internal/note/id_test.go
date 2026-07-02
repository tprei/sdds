package note

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewIDReturnsUUIDV7(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("create id: %v", err)
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("uuid version = %d, want 7", parsed.Version())
	}
}

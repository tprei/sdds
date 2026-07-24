package comment

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewIDCreatesUUIDv7(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("new comment id: %v", err)
	}

	parsed, err := uuid.Parse(string(id))
	if err != nil {
		t.Fatalf("parse comment id: %v", err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("comment id version = %d, want 7", parsed.Version())
	}
}

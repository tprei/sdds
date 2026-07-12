package user

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/author"
)

func TestNewIDsCreateUUIDv7Values(t *testing.T) {
	tests := []struct {
		name   string
		create func() (string, error)
	}{
		{
			name: "user",
			create: func() (string, error) {
				id, err := NewUserID()
				return string(id), err
			},
		},
		{
			name: "author",
			create: func() (string, error) {
				id, err := author.NewAuthorID()
				return string(id), err
			},
		},
		{
			name: "login identity",
			create: func() (string, error) {
				id, err := NewLoginIdentityID()
				return string(id), err
			},
		},
		{
			name: "session",
			create: func() (string, error) {
				id, err := NewSessionID()
				return string(id), err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.create()
			if err != nil {
				t.Fatalf("create id: %v", err)
			}
			parsed, err := uuid.Parse(id)
			if err != nil {
				t.Fatalf("parse uuid: %v", err)
			}
			if parsed.Version() != 7 {
				t.Fatalf("uuid version = %d, want 7", parsed.Version())
			}
		})
	}
}

package user

import (
	"fmt"

	"github.com/google/uuid"
)

func NewUserID() (UserID, error) {
	id, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	return UserID(id), nil
}

func NewLoginIdentityID() (LoginIdentityID, error) {
	id, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	return LoginIdentityID(id), nil
}

func NewSessionID() (SessionID, error) {
	id, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	return SessionID(id), nil
}

func NewContactChannelID() (ContactChannelID, error) {
	id, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	return ContactChannelID(id), nil
}

func NewContactChannelTokenID() (ContactChannelTokenID, error) {
	id, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	return ContactChannelTokenID(id), nil
}

func newUUIDv7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("create uuid v7: %w", err)
	}
	return id.String(), nil
}

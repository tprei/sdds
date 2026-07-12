package user

import (
	"context"
	"errors"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
)

var (
	ErrUsernameTaken      = errors.New("username taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionRevoked     = errors.New("session revoked")
	ErrUserDisabled       = errors.New("user disabled")
)

type UserID string
type LoginIdentityID string
type SessionID string

type UserState string

const (
	UserStateActive   UserState = "active"
	UserStateDisabled UserState = "disabled"
)

type LoginIdentityKind string

const (
	LoginIdentityKindPassword = "password"
	LoginIdentityKindOIDC     = "oidc"
)

const LoginIdentityProviderLocal = "local"

type User struct {
	ID        UserID
	State     UserState
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Author struct {
	ID          author.AuthorID
	UserID      UserID
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LoginIdentity struct {
	ID                   LoginIdentityID
	UserID               UserID
	Kind                 LoginIdentityKind
	Provider             string
	NormalizedIdentifier string
	SecretHash           *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Session struct {
	ID        SessionID
	UserID    UserID
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type CurrentSession struct {
	Session  Session
	User     User
	Author   Author
	Username string
}

type PasswordLogin struct {
	User       User
	Author     Author
	Username   string
	SecretHash string
}

type CreateUserInput struct {
	Username    string
	Password    string
	DisplayName string
}

type LoginInput struct {
	Username string
	Password string
}

type CreatePasswordUserInput struct {
	Username    string
	DisplayName string
	SecretHash  string
	TokenHash   string
	ExpiresAt   time.Time
}

type CreateSessionInput struct {
	UserID    UserID
	TokenHash string
	ExpiresAt time.Time
}

type Store interface {
	CreatePasswordUser(ctx context.Context, input CreatePasswordUserInput) (CurrentSession, error)
	FindPasswordLogin(ctx context.Context, normalizedUsername string) (PasswordLogin, error)
	CreateSession(ctx context.Context, input CreateSessionInput) (CurrentSession, error)
	FindCurrentSession(ctx context.Context, tokenHash string, now time.Time) (CurrentSession, error)
	RevokeSession(ctx context.Context, sessionID SessionID, revokedAt time.Time) error
	FindAuthorByUserID(ctx context.Context, userID UserID) (Author, error)
}

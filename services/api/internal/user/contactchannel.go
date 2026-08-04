package user

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
)

type ContactChannelID string
type ContactChannel string
type ContactChannelTokenID string
type ContactChannelTokenPurpose string

const (
	ContactChannelEmail ContactChannel = "email"

	ContactChannelTokenPurposeVerify ContactChannelTokenPurpose = "verify"
	ContactChannelTokenPurposeReset  ContactChannelTokenPurpose = "reset"

	// ContactChannelVerifiedViaToken marks an address verified by an emailed token.
	ContactChannelVerifiedViaToken = "token"

	EmailVerificationTokenLifetime = 24 * time.Hour
	PasswordResetTokenLifetime     = time.Hour

	EmailMaxLength = 254
)

var (
	ErrContactChannelNotFound        = errors.New("contact channel not found")
	ErrContactChannelTokenInvalid    = errors.New("contact channel token invalid")
	ErrContactChannelTokenConflict   = errors.New("contact channel token conflict")
	ErrContactChannelAlreadyVerified = errors.New("contact channel value already verified")
)

// ContactChannelRecord is one address row held by a user. VerifiedAt is nil
// until the address is confirmed; VerifiedVia records who set verified state.
type ContactChannelRecord struct {
	ID          ContactChannelID
	UserID      UserID
	Channel     ContactChannel
	Value       string
	VerifiedAt  *time.Time
	VerifiedVia string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateContactChannelTokenInput struct {
	ID        ContactChannelTokenID
	ChannelID ContactChannelID
	Purpose   ContactChannelTokenPurpose
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ContactChannelStore persists addresses and their verification/reset tokens.
// It is separate from Store so the two storage surfaces evolve independently.
type ContactChannelStore interface {
	UpsertUnverifiedEmail(ctx context.Context, userID UserID, normalizedValue string, now time.Time) (ContactChannelRecord, error)
	FindEmailForUser(ctx context.Context, userID UserID) (ContactChannelRecord, error)
	FindPendingEmailForUser(ctx context.Context, userID UserID) (ContactChannelRecord, error)
	FindVerifiedEmail(ctx context.Context, normalizedValue string) (ContactChannelRecord, error)
	CreateToken(ctx context.Context, input CreateContactChannelTokenInput) error
	ConsumeTokenAndMarkVerified(ctx context.Context, tokenHash string, verifiedVia string, now time.Time) (ContactChannelRecord, error)
	ConsumeTokenAndSetPassword(ctx context.Context, tokenHash string, secretHash string, now time.Time) (ContactChannelRecord, error)
}

// NormalizeEmail lowercases and trims whitespace. Provider-specific casing
// rules (Gmail dot-folding, plus-tag stripping) are intentionally not applied.
func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// ValidateEmail checks the normalized address shape: a single @ that is neither
// leading nor trailing, no embedded spaces, and at most 254 characters. It
// performs no DNS or mailbox existence check.
func ValidateEmail(normalized string) []ValidationProblem {
	if normalized == "" {
		return []ValidationProblem{{Field: "email", Code: "required"}}
	}
	if len(normalized) > EmailMaxLength {
		return []ValidationProblem{{Field: "email", Code: "too_long"}}
	}
	if !validEmailShape(normalized) {
		return []ValidationProblem{{Field: "email", Code: "invalid"}}
	}
	return nil
}

func validEmailShape(value string) bool {
	if strings.Count(value, "@") != 1 {
		return false
	}
	at := strings.IndexByte(value, '@')
	if at == 0 || at == len(value)-1 {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// NewContactChannelToken returns a random single-use token shared with sessions.
func NewContactChannelToken() (string, error) {
	return newRandomToken()
}

// HashContactChannelToken hashes a contact-channel token for storage, mirroring
// session token hashing (sha256 hex).
func HashContactChannelToken(token string) string {
	return hashToken(token)
}

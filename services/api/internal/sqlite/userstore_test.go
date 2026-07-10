package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestUserStoreCreatesPasswordUserWithAuthorIdentityAndSession(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	tokenHash := user.HashSessionToken("session-token")

	current, err := store.CreatePasswordUser(ctx, user.CreatePasswordUserInput{
		Username:    "thiago",
		DisplayName: "Thiago",
		SecretHash:  "password-hash",
		TokenHash:   tokenHash,
		ExpiresAt:   now.Add(user.SessionLifetime),
	})
	if err != nil {
		t.Fatalf("create password user: %v", err)
	}

	if current.User.ID == "" {
		t.Fatal("user id is empty")
	}
	if current.Author.ID == "" {
		t.Fatal("author id is empty")
	}
	if current.Session.ID == "" {
		t.Fatal("session id is empty")
	}
	if current.Session.TokenHash != tokenHash {
		t.Fatalf("token hash = %q, want %q", current.Session.TokenHash, tokenHash)
	}
	if current.Username != "thiago" {
		t.Fatalf("username = %q, want thiago", current.Username)
	}
	if current.Author.UserID != current.User.ID {
		t.Fatalf("author user id = %q, want %q", current.Author.UserID, current.User.ID)
	}

	login, err := store.FindPasswordLogin(ctx, "thiago")
	if err != nil {
		t.Fatalf("find password login: %v", err)
	}
	if login.SecretHash != "password-hash" {
		t.Fatalf("secret hash = %q, want password-hash", login.SecretHash)
	}
	if diff := cmp.Diff(current.User, login.User); diff != "" {
		t.Fatalf("login user mismatch (-want +got):\n%s", diff)
	}

	found, err := store.FindCurrentSession(ctx, tokenHash, now)
	if err != nil {
		t.Fatalf("find current session: %v", err)
	}
	if diff := cmp.Diff(current, found); diff != "" {
		t.Fatalf("current session mismatch (-want +got):\n%s", diff)
	}
}

func TestUserStoreReturnsUsernameTakenAndRollsBackDuplicateUser(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	input := user.CreatePasswordUserInput{
		Username:    "thiago",
		DisplayName: "Thiago",
		SecretHash:  "password-hash",
		TokenHash:   user.HashSessionToken("session-token-a"),
		ExpiresAt:   now.Add(user.SessionLifetime),
	}

	if _, err := store.CreatePasswordUser(ctx, input); err != nil {
		t.Fatalf("create first password user: %v", err)
	}
	input.TokenHash = user.HashSessionToken("session-token-b")
	_, err := store.CreatePasswordUser(ctx, input)
	if !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("create duplicate user error = %v, want ErrUsernameTaken", err)
	}

	var userCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("user count = %d, want 1", userCount)
	}
}

func TestUserStoreFindsMissingPasswordLoginAsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	store := NewUserStore(openMigratedDatabase(t, ctx))

	_, err := store.FindPasswordLogin(ctx, "missing")
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("find password login error = %v, want ErrInvalidCredentials", err)
	}
}

func TestUserStoreCreatesAdditionalSessionsForActiveUsers(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	current := createTestPasswordUser(t, ctx, store, now, "thiago", "session-token-a")

	nextHash := user.HashSessionToken("session-token-b")
	next, err := store.CreateSession(ctx, user.CreateSessionInput{
		UserID:    current.User.ID,
		TokenHash: nextHash,
		ExpiresAt: now.Add(user.SessionLifetime),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if next.Session.TokenHash != nextHash {
		t.Fatalf("token hash = %q, want %q", next.Session.TokenHash, nextHash)
	}
	if next.User.ID != current.User.ID {
		t.Fatalf("session user id = %q, want %q", next.User.ID, current.User.ID)
	}
}

func TestUserStoreRejectsExpiredSessionCreationWithoutInsertingRows(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	current := createTestPasswordUser(t, ctx, store, now, "thiago", "session-token-a")
	expiredHash := user.HashSessionToken("expired-token")

	_, err := store.CreateSession(ctx, user.CreateSessionInput{
		UserID:    current.User.ID,
		TokenHash: expiredHash,
		ExpiresAt: now,
	})
	if !errors.Is(err, user.ErrSessionExpired) {
		t.Fatalf("create expired session error = %v, want ErrSessionExpired", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, expiredHash).Scan(&count); err != nil {
		t.Fatalf("count expired sessions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expired session count = %d, want 0", count)
	}
}

func TestUserStoreRejectsExpiredPasswordUserSessionWithoutInsertingRows(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })

	_, err := store.CreatePasswordUser(ctx, user.CreatePasswordUserInput{
		Username:    "expired",
		DisplayName: "Expired",
		SecretHash:  "password-hash",
		TokenHash:   user.HashSessionToken("expired-token"),
		ExpiresAt:   now,
	})
	if !errors.Is(err, user.ErrSessionExpired) {
		t.Fatalf("create expired password user error = %v, want ErrSessionExpired", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("user count = %d, want 0", count)
	}
}

func TestUserStoreRejectsExpiredRevokedAndDisabledSessions(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })

	expiredHash := user.HashSessionToken("expired-token")
	createTestPasswordUserWithExpiry(t, ctx, store, now, "expired", "Expired", expiredHash, now.Add(user.SessionLifetime))
	if _, err := db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, unixMillis(now.Add(-time.Minute)), expiredHash); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	_, err := store.FindCurrentSession(ctx, expiredHash, now)
	if !errors.Is(err, user.ErrSessionExpired) {
		t.Fatalf("expired session error = %v, want ErrSessionExpired", err)
	}

	revoked := createTestPasswordUser(t, ctx, store, now, "revoked", "revoked-token")
	if err := store.RevokeSession(ctx, revoked.Session.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if err := store.RevokeSession(ctx, revoked.Session.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("revoke session again: %v", err)
	}
	_, err = store.FindCurrentSession(ctx, user.HashSessionToken("revoked-token"), now)
	if !errors.Is(err, user.ErrSessionRevoked) {
		t.Fatalf("revoked session error = %v, want ErrSessionRevoked", err)
	}

	disabled := createTestPasswordUser(t, ctx, store, now, "disabled", "disabled-token")
	if _, err := db.ExecContext(ctx, `UPDATE users SET state = ? WHERE id = ?`, user.UserStateDisabled, disabled.User.ID); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	_, err = store.FindCurrentSession(ctx, user.HashSessionToken("disabled-token"), now)
	if !errors.Is(err, user.ErrUserDisabled) {
		t.Fatalf("disabled session error = %v, want ErrUserDisabled", err)
	}
	_, err = store.CreateSession(ctx, user.CreateSessionInput{
		UserID:    disabled.User.ID,
		TokenHash: user.HashSessionToken("new-disabled-token"),
		ExpiresAt: now.Add(user.SessionLifetime),
	})
	if !errors.Is(err, user.ErrUserDisabled) {
		t.Fatalf("disabled create session error = %v, want ErrUserDisabled", err)
	}
}

func createTestPasswordUser(t *testing.T, ctx context.Context, store *UserStore, now time.Time, username string, token string) user.CurrentSession {
	t.Helper()
	return createTestPasswordUserWithExpiry(t, ctx, store, now, username, username, user.HashSessionToken(token), now.Add(user.SessionLifetime))
}

func createTestPasswordUserWithExpiry(
	t *testing.T,
	ctx context.Context,
	store *UserStore,
	now time.Time,
	username string,
	displayName string,
	tokenHash string,
	expiresAt time.Time,
) user.CurrentSession {
	t.Helper()
	current, err := store.CreatePasswordUser(ctx, user.CreatePasswordUserInput{
		Username:    username,
		DisplayName: displayName,
		SecretHash:  "password-hash-" + username,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("create password user %s: %v", username, err)
	}
	return current
}

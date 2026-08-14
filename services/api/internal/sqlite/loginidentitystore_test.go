package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tprei/sdds/services/api/internal/user"
)

func oidcTestNow() time.Time {
	return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
}

func oidcInput(provider, subject, username, token string, now time.Time) user.ResolveOIDCIdentityInput {
	return user.ResolveOIDCIdentityInput{
		Provider:  provider,
		Subject:   subject,
		Username:  username,
		TokenHash: user.HashSessionToken(token),
		ExpiresAt: now.Add(user.SessionLifetime),
	}
}

func TestResolveOIDCIdentityExistingIdentitySignsInAsSameUser(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })

	firstInput := oidcInput(user.LoginIdentityProviderGoogle, "subject-1", "alice", "token-1", now)
	first, err := store.ResolveOIDCIdentity(ctx, firstInput)
	if err != nil {
		t.Fatalf("first oidc sign in: %v", err)
	}

	secondInput := firstInput
	secondInput.Username = ""
	secondInput.TokenHash = user.HashSessionToken("token-2")
	second, err := store.ResolveOIDCIdentity(ctx, secondInput)
	if err != nil {
		t.Fatalf("second oidc sign in: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("second user id = %q, want %q", second.User.ID, first.User.ID)
	}
	if second.Username != "alice" {
		t.Fatalf("second username = %q, want alice", second.Username)
	}

	var userCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("user count = %d, want 1", userCount)
	}
}

func TestResolveOIDCIdentityLinksVerifiedGoogleEmail(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	passwordUser := createTestPasswordUser(t, ctx, store, now, "alice", "password-token")
	insertVerifiedEmail(t, ctx, db, passwordUser.User.ID, "alice@example.com", "token")

	current, err := store.ResolveOIDCIdentity(ctx, user.ResolveOIDCIdentityInput{
		Provider:      user.LoginIdentityProviderGoogle,
		Subject:       "google-subject",
		Email:         "alice@example.com",
		EmailVerified: true,
		TokenHash:     user.HashSessionToken("google-token"),
		ExpiresAt:     now.Add(user.SessionLifetime),
	})
	if err != nil {
		t.Fatalf("link google identity: %v", err)
	}
	if current.User.ID != passwordUser.User.ID {
		t.Fatalf("linked user id = %q, want %q", current.User.ID, passwordUser.User.ID)
	}
	var identityCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_login_identities WHERE user_id = ?`, passwordUser.User.ID).Scan(&identityCount); err != nil {
		t.Fatalf("count linked identities: %v", err)
	}
	if identityCount != 2 {
		t.Fatalf("linked identity count = %d, want 2", identityCount)
	}
}

func TestResolveOIDCIdentityDoesNotLinkUnsafeEmailMatches(t *testing.T) {
	cases := []struct {
		name          string
		provider      string
		emailVerified bool
		channelVerify bool
	}{
		{name: "provider email unverified", provider: user.LoginIdentityProviderGoogle, emailVerified: false, channelVerify: true},
		{name: "channel unverified", provider: user.LoginIdentityProviderGoogle, emailVerified: true, channelVerify: false},
		{name: "provider not allowlisted", provider: "custom", emailVerified: true, channelVerify: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			now := oidcTestNow()
			db := openMigratedDatabase(t, ctx)
			store := newUserStore(db, func() time.Time { return now })
			passwordUser := createTestPasswordUser(t, ctx, store, now, "alice", "password-token")
			insertEmail(t, ctx, db, passwordUser.User.ID, "alice@example.com", test.channelVerify)

			input := oidcInput(test.provider, "new-subject", "new-user", "oidc-token", now)
			input.Email = "alice@example.com"
			input.EmailVerified = test.emailVerified
			_, err := store.ResolveOIDCIdentity(ctx, input)
			if test.provider == "custom" && test.emailVerified && test.channelVerify {
				if err == nil {
					t.Fatal("custom provider unexpectedly linked or created a verified duplicate")
				}
			} else if err != nil {
				t.Fatalf("resolve unsafe email match: %v", err)
			}

			var linkedCount int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_login_identities WHERE user_id = ? AND provider = ?`, passwordUser.User.ID, test.provider).Scan(&linkedCount); err != nil {
				t.Fatalf("count linked identities: %v", err)
			}
			if linkedCount != 0 {
				t.Fatalf("unsafe email linked %d identities", linkedCount)
			}
		})
	}
}

func TestResolveOIDCIdentityCreatesWithDisplayNameOrUsername(t *testing.T) {
	cases := []struct {
		name        string
		displayName string
		wantName    string
	}{
		{name: "display name", displayName: "Aline Pessoa", wantName: "Aline Pessoa"},
		{name: "username fallback", wantName: "aline"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			now := oidcTestNow()
			db := openMigratedDatabase(t, ctx)
			store := newUserStore(db, func() time.Time { return now })
			input := oidcInput(user.LoginIdentityProviderApple, test.name, "aline", "token-"+test.name, now)
			input.DisplayName = test.displayName
			current, err := store.ResolveOIDCIdentity(ctx, input)
			if err != nil {
				t.Fatalf("create oidc identity: %v", err)
			}
			if current.Author.DisplayName != test.wantName {
				t.Fatalf("display name = %q, want %q", current.Author.DisplayName, test.wantName)
			}
		})
	}
}

func TestResolveOIDCIdentityTruncatesDisplayNameToSixtyRunes(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	store := newUserStore(openMigratedDatabase(t, ctx), func() time.Time { return now })
	input := oidcInput(user.LoginIdentityProviderGoogle, "long-name", "long-user", "long-token", now)
	input.DisplayName = strings.Repeat("😀", 70)

	current, err := store.ResolveOIDCIdentity(ctx, input)
	if err != nil {
		t.Fatalf("create long display name: %v", err)
	}
	if got := utf8.RuneCountInString(current.Author.DisplayName); got != 60 {
		t.Fatalf("display name rune count = %d, want 60", got)
	}
}

func TestResolveOIDCIdentityTakenUsernameReturnsUsernameTaken(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	if _, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderGoogle, "subject-a", "alice", "token-a", now)); err != nil {
		t.Fatalf("create first oidc identity: %v", err)
	}
	_, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderApple, "subject-b", "alice", "token-b", now))
	if !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("taken username error = %v, want ErrUsernameTaken", err)
	}
	var userCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("user count = %d, want 1", userCount)
	}
}

func TestResolveOIDCIdentityEmptyUsernameRollsBackWithoutUsers(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	input := oidcInput(user.LoginIdentityProviderGoogle, "subject-empty", "", "token-empty", now)
	_, err := store.ResolveOIDCIdentity(ctx, input)
	if !errors.Is(err, user.ErrUsernameRequired) {
		t.Fatalf("empty username error = %v, want ErrUsernameRequired", err)
	}
	var userCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("user count = %d, want 0", userCount)
	}
}

func TestResolveOIDCIdentityAppleRelayCreatesSeparateAccount(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	firstInput := oidcInput(user.LoginIdentityProviderApple, "apple-subject-a", "alice", "apple-token-a", now)
	firstInput.Email = "relay-a@privaterelay.appleid.com"
	firstInput.EmailVerified = true
	first, err := store.ResolveOIDCIdentity(ctx, firstInput)
	if err != nil {
		t.Fatalf("create first relay account: %v", err)
	}
	secondInput := oidcInput(user.LoginIdentityProviderApple, "apple-subject-b", "beatriz", "apple-token-b", now)
	secondInput.Email = "relay-b@privaterelay.appleid.com"
	secondInput.EmailVerified = true
	second, err := store.ResolveOIDCIdentity(ctx, secondInput)
	if err != nil {
		t.Fatalf("create second relay account: %v", err)
	}
	if first.User.ID == second.User.ID {
		t.Fatal("relay addresses resolved to the same user")
	}
	var verifiedCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_contact_channels WHERE channel = 'email' AND verified_at IS NOT NULL`).Scan(&verifiedCount); err != nil {
		t.Fatalf("count verified relay addresses: %v", err)
	}
	if verifiedCount != 2 {
		t.Fatalf("verified relay address count = %d, want 2", verifiedCount)
	}
}

func TestListLoginIdentitiesExcludesUsernameOnlyRowAndOrdersByCreatedAtAndID(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	current, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderGoogle, "subject-list", "alice", "token-list", now))
	if err != nil {
		t.Fatalf("create list fixture: %v", err)
	}
	for _, identity := range []struct {
		id        string
		createdAt int64
	}{
		{id: "identity-z", createdAt: 1},
		{id: "identity-a", createdAt: 1},
	} {
		if _, err := db.ExecContext(ctx, `INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at) VALUES (?, ?, 'oidc', 'google', ?, NULL, ?, ?)`, identity.id, current.User.ID, identity.id, identity.createdAt, identity.createdAt); err != nil {
			t.Fatalf("insert list fixture %s: %v", identity.id, err)
		}
	}

	identities, err := store.ListLoginIdentities(ctx, current.User.ID)
	if err != nil {
		t.Fatalf("list login identities: %v", err)
	}
	if len(identities) != 3 {
		t.Fatalf("identity count = %d, want 3", len(identities))
	}
	wantIDs := []user.LoginIdentityID{"identity-a", "identity-z", identities[2].ID}
	for index, wantID := range wantIDs {
		if identities[index].ID != wantID {
			t.Fatalf("identity %d = %q, want %q", index, identities[index].ID, wantID)
		}
	}
	for _, identity := range identities {
		if identity.ID == "" || (identity.Provider == user.LoginIdentityProviderLocal && identity.Kind == user.LoginIdentityKindOIDC) {
			t.Fatalf("username-only local identity was listed: %+v", identity)
		}
	}
}

func TestDeleteLoginIdentityRejectsLastMethod(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	current, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderGoogle, "subject-delete-last", "alice", "token-delete-last", now))
	if err != nil {
		t.Fatalf("create delete fixture: %v", err)
	}
	identities, err := store.ListLoginIdentities(ctx, current.User.ID)
	if err != nil {
		t.Fatalf("list delete fixture: %v", err)
	}
	err = store.DeleteLoginIdentity(ctx, current.User.ID, identities[0].ID)
	if !errors.Is(err, user.ErrLastLoginIdentity) {
		t.Fatalf("last identity error = %v, want ErrLastLoginIdentity", err)
	}
}

func TestDeleteLoginIdentityRejectsOtherUserAndMissingIdentity(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	first, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderGoogle, "subject-delete-a", "alice", "token-delete-a", now))
	if err != nil {
		t.Fatalf("create first delete fixture: %v", err)
	}
	second, err := store.ResolveOIDCIdentity(ctx, oidcInput(user.LoginIdentityProviderGoogle, "subject-delete-b", "beatriz", "token-delete-b", now))
	if err != nil {
		t.Fatalf("create second delete fixture: %v", err)
	}
	secondIdentities, err := store.ListLoginIdentities(ctx, second.User.ID)
	if err != nil {
		t.Fatalf("list second delete fixture: %v", err)
	}
	if err := store.DeleteLoginIdentity(ctx, first.User.ID, secondIdentities[0].ID); !errors.Is(err, user.ErrLoginIdentityNotFound) {
		t.Fatalf("other user's identity error = %v, want ErrLoginIdentityNotFound", err)
	}
	if err := store.DeleteLoginIdentity(ctx, first.User.ID, "missing-identity"); !errors.Is(err, user.ErrLoginIdentityNotFound) {
		t.Fatalf("missing identity error = %v, want ErrLoginIdentityNotFound", err)
	}
}

func TestDeleteLoginIdentityDeletesExactlyRequestedRow(t *testing.T) {
	ctx := context.Background()
	now := oidcTestNow()
	db := openMigratedDatabase(t, ctx)
	store := newUserStore(db, func() time.Time { return now })
	passwordUser := createTestPasswordUser(t, ctx, store, now, "alice", "password-delete")
	insertVerifiedEmail(t, ctx, db, passwordUser.User.ID, "alice@example.com", "token")
	if _, err := store.ResolveOIDCIdentity(ctx, user.ResolveOIDCIdentityInput{
		Provider:      user.LoginIdentityProviderGoogle,
		Subject:       "google-delete",
		Email:         "alice@example.com",
		EmailVerified: true,
		TokenHash:     user.HashSessionToken("google-delete"),
		ExpiresAt:     now.Add(user.SessionLifetime),
	}); err != nil {
		t.Fatalf("link delete fixture: %v", err)
	}
	identities, err := store.ListLoginIdentities(ctx, passwordUser.User.ID)
	if err != nil {
		t.Fatalf("list delete identities: %v", err)
	}
	if len(identities) != 2 {
		t.Fatalf("identity count before delete = %d, want 2", len(identities))
	}
	var deleteID user.LoginIdentityID
	for _, identity := range identities {
		if identity.Provider == user.LoginIdentityProviderGoogle {
			deleteID = identity.ID
		}
	}
	if deleteID == "" {
		t.Fatal("google identity not found")
	}
	if err := store.DeleteLoginIdentity(ctx, passwordUser.User.ID, deleteID); err != nil {
		t.Fatalf("delete google identity: %v", err)
	}
	remaining, err := store.ListLoginIdentities(ctx, passwordUser.User.ID)
	if err != nil {
		t.Fatalf("list remaining identities: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Provider != user.LoginIdentityProviderLocal {
		t.Fatalf("remaining identities = %+v, want local password identity", remaining)
	}
	var deletedCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_login_identities WHERE id = ?`, deleteID).Scan(&deletedCount); err != nil {
		t.Fatalf("count deleted identity: %v", err)
	}
	if deletedCount != 0 {
		t.Fatalf("deleted identity count = %d, want 0", deletedCount)
	}
}

func insertEmail(t *testing.T, ctx context.Context, db *sql.DB, userID user.UserID, value string, verified bool) {
	t.Helper()
	verifiedAt := any(nil)
	verifiedVia := any(nil)
	if verified {
		verifiedAt = int64(1)
		verifiedVia = "token"
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES (?, ?, 'email', ?, ?, ?, 0, 0)`, "channel-"+string(userID), userID, value, verifiedAt, verifiedVia); err != nil {
		t.Fatalf("insert email: %v", err)
	}
}

func insertVerifiedEmail(t *testing.T, ctx context.Context, db *sql.DB, userID user.UserID, value string, via string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES (?, ?, 'email', ?, 1, ?, 0, 0)`, "verified-"+string(userID), userID, value, via); err != nil {
		t.Fatalf("insert verified email: %v", err)
	}
}

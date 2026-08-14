package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
)

func newContactChannelStoreTestClock() (func() time.Time, *time.Time) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return now }, &now
}

func seedContactChannelUser(t *testing.T, ctx context.Context, db execer, userID, username string) {
	t.Helper()
	insertAuthorStoreUser(t, ctx, db, user.UserID(userID), author.AuthorID("author-"+userID), username)
	if _, err := db.ExecContext(ctx, `INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at) VALUES (?, ?, 'password', 'local', ?, 'old-hash', 0, 0)`, "identity-"+userID, userID, username); err != nil {
		t.Fatalf("insert identity %s: %v", userID, err)
	}
}

// markChannelVerified sets verified state on a channel without the token flow,
// for test setup only. It mirrors markContactChannelVerifiedSQL.
func markChannelVerified(t *testing.T, ctx context.Context, db execer, channelID user.ContactChannelID, now time.Time) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `UPDATE user_contact_channels SET verified_at = COALESCE(verified_at, ?), verified_via = COALESCE(verified_via, ?), updated_at = ? WHERE id = ?`, unixMillis(now), user.ContactChannelVerifiedViaToken, unixMillis(now), channelID); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
}

func TestContactChannelUpsertReplacesPendingButKeepsVerified(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, now := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")

	first, err := store.UpsertUnverifiedEmail(ctx, "user-a", "old@example.com", clock())
	if err != nil {
		t.Fatalf("upsert first: %v", err)
	}
	markChannelVerified(t, ctx, db, first.ID, clock())

	second, err := store.UpsertUnverifiedEmail(ctx, "user-a", "new@example.com", clock())
	if err != nil {
		t.Fatalf("upsert second: %v", err)
	}
	if second.Value != "new@example.com" {
		t.Fatalf("second value = %q, want new@example.com", second.Value)
	}

	// FindEmailForUser returns the verified address first; the new pending one survives.
	email, err := store.FindEmailForUser(ctx, "user-a")
	if err != nil {
		t.Fatalf("find email: %v", err)
	}
	if email.Value != "old@example.com" || email.VerifiedAt == nil {
		t.Fatalf("email = {%s, verified=%v}, want verified old@example.com", email.Value, email.VerifiedAt)
	}

	total := countRows(t, ctx, db, `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = 'user-a' AND channel = 'email'`)
	if total != 2 {
		t.Fatalf("channel rows = %d, want 2 (verified + pending)", total)
	}
	pending := countRows(t, ctx, db, `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = 'user-a' AND channel = 'email' AND verified_at IS NULL`)
	if pending != 1 {
		t.Fatalf("pending rows = %d, want 1", pending)
	}

	// A different pending value replaces the previous pending row; the verified row stays.
	*now = clock().Add(time.Hour)
	if _, err := store.UpsertUnverifiedEmail(ctx, "user-a", "other@example.com", clock()); err != nil {
		t.Fatalf("upsert third: %v", err)
	}
	totalAfter := countRows(t, ctx, db, `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = 'user-a' AND channel = 'email'`)
	if totalAfter != 2 {
		t.Fatalf("channel rows after replace = %d, want 2 (verified + new pending)", totalAfter)
	}
}

func TestContactChannelUpsertSameValueIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, _ := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")

	first, err := store.UpsertUnverifiedEmail(ctx, "user-a", "ana@example.com", clock())
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := store.UpsertUnverifiedEmail(ctx, "user-a", "ana@example.com", clock())
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent upsert changed id: %q -> %q", first.ID, second.ID)
	}
}

func TestContactChannelConsumeTokenSingleUseAndExpiry(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, now := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")

	channel, err := store.UpsertUnverifiedEmail(ctx, "user-a", "ana@example.com", clock())
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	tokenID, _ := user.NewContactChannelTokenID()
	tokenHash := "verify-hash-1"
	expires := clock().Add(user.EmailVerificationTokenLifetime)
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: tokenID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: tokenHash, CreatedAt: clock(), ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("create token: %v", err)
	}

	verified, err := store.ConsumeTokenAndMarkVerified(ctx, tokenHash, user.ContactChannelVerifiedViaToken, clock())
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if verified.VerifiedAt == nil {
		t.Fatal("channel not marked verified after consume")
	}
	if _, err := store.ConsumeTokenAndMarkVerified(ctx, tokenHash, user.ContactChannelVerifiedViaToken, clock()); !errors.Is(err, user.ErrContactChannelTokenInvalid) {
		t.Fatalf("replay consume err = %v, want ErrContactChannelTokenInvalid", err)
	}

	expiredID, _ := user.NewContactChannelTokenID()
	expiredHash := "verify-hash-expired"
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: expiredID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: expiredHash, CreatedAt: clock(), ExpiresAt: clock(),
	}); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	*now = clock().Add(2 * time.Hour)
	if _, err := store.ConsumeTokenAndMarkVerified(ctx, expiredHash, user.ContactChannelVerifiedViaToken, clock()); !errors.Is(err, user.ErrContactChannelTokenInvalid) {
		t.Fatalf("expired consume err = %v, want ErrContactChannelTokenInvalid", err)
	}
}

func TestContactChannelVerifyRejectsAddressHeldByAnother(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, _ := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")
	seedContactChannelUser(t, ctx, db, "user-b", "bruno")

	a, err := store.UpsertUnverifiedEmail(ctx, "user-a", "shared@example.com", clock())
	if err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	markChannelVerified(t, ctx, db, a.ID, clock())

	b, err := store.UpsertUnverifiedEmail(ctx, "user-b", "shared@example.com", clock())
	if err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	bTokenID, _ := user.NewContactChannelTokenID()
	bTokenHash := "verify-hash-b"
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: bTokenID, ChannelID: b.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: bTokenHash, CreatedAt: clock(), ExpiresAt: clock().Add(user.EmailVerificationTokenLifetime),
	}); err != nil {
		t.Fatalf("create b token: %v", err)
	}
	if _, err := store.ConsumeTokenAndMarkVerified(ctx, bTokenHash, user.ContactChannelVerifiedViaToken, clock()); !errors.Is(err, user.ErrContactChannelAlreadyVerified) {
		t.Fatalf("verify b err = %v, want ErrContactChannelAlreadyVerified", err)
	}
	// The failed verify must not burn the token: it stays unconsumed.
	var consumed *int64
	if err := db.QueryRowContext(ctx, `SELECT consumed_at FROM user_contact_channel_tokens WHERE token_hash = ?`, bTokenHash).Scan(&consumed); err != nil {
		t.Fatalf("read b token: %v", err)
	}
	if consumed != nil {
		t.Fatal("b token was consumed despite the failed verify")
	}
}

func TestContactChannelSetPasswordCreatesIdentityRevokesSessionsConsumesAllTokens(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, _ := newContactChannelStoreTestClock()

	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES ('user-x', 'active', 0, 0)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO authors (id, user_id, display_name, created_at, updated_at) VALUES ('author-x', 'user-x', 'X', 0, 0)`); err != nil {
		t.Fatalf("insert author: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at) VALUES ('identity-x', 'user-x', 'oidc', 'local', 'x', NULL, 0, 0)`); err != nil {
		t.Fatalf("insert local identity: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at, revoked_at) VALUES ('sess-1', 'user-x', 'hash-1', 0, 9999999999, NULL)`); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	channel, err := store.UpsertUnverifiedEmail(ctx, "user-x", "x@example.com", clock())
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}
	markChannelVerified(t, ctx, db, channel.ID, clock())

	resetID, _ := user.NewContactChannelTokenID()
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: resetID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeReset,
		TokenHash: "reset-hash-1", CreatedAt: clock(), ExpiresAt: clock().Add(user.PasswordResetTokenLifetime),
	}); err != nil {
		t.Fatalf("create reset token: %v", err)
	}
	// An outstanding verification token must also be invalidated by the reset.
	verifyID, _ := user.NewContactChannelTokenID()
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: verifyID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: "verify-hash-x", CreatedAt: clock(), ExpiresAt: clock().Add(user.EmailVerificationTokenLifetime),
	}); err != nil {
		t.Fatalf("create verify token: %v", err)
	}

	if _, err := store.ConsumeTokenAndSetPassword(ctx, "reset-hash-1", "new-hash", clock()); err != nil {
		t.Fatalf("set password: %v", err)
	}

	var identityCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_login_identities WHERE user_id = 'user-x' AND kind = 'password'`).Scan(&identityCount); err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if identityCount != 1 {
		t.Fatalf("password identity count = %d, want 1", identityCount)
	}

	var secretHash string
	if err := db.QueryRowContext(ctx, `SELECT secret_hash FROM user_login_identities WHERE user_id = 'user-x'`).Scan(&secretHash); err != nil {
		t.Fatalf("read secret_hash: %v", err)
	}
	if secretHash != "new-hash" {
		t.Fatalf("secret_hash = %q, want new-hash", secretHash)
	}

	var revokedAt *int64
	if err := db.QueryRowContext(ctx, `SELECT revoked_at FROM sessions WHERE id = 'sess-1'`).Scan(&revokedAt); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if revokedAt == nil {
		t.Fatal("session was not revoked")
	}

	var resetConsumed *int64
	if err := db.QueryRowContext(ctx, `SELECT consumed_at FROM user_contact_channel_tokens WHERE token_hash = 'reset-hash-1'`).Scan(&resetConsumed); err != nil {
		t.Fatalf("read reset token: %v", err)
	}
	if resetConsumed == nil {
		t.Fatal("reset token was not consumed")
	}

	var verifyConsumed *int64
	if err := db.QueryRowContext(ctx, `SELECT consumed_at FROM user_contact_channel_tokens WHERE token_hash = 'verify-hash-x'`).Scan(&verifyConsumed); err != nil {
		t.Fatalf("read verify token: %v", err)
	}
	if verifyConsumed == nil {
		t.Fatal("outstanding verification token was not consumed by the reset")
	}
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func countRows(t *testing.T, ctx context.Context, db rowQuerier, query string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestContactChannelSetPasswordMonotonicCredentialVersionClosesLoginRace(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, now := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")

	channel, err := store.UpsertUnverifiedEmail(ctx, "user-a", "ana@example.com", clock())
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}
	markChannelVerified(t, ctx, db, channel.ID, clock())

	resetID, _ := user.NewContactChannelTokenID()
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: resetID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeReset,
		TokenHash: "reset-hash", CreatedAt: clock(), ExpiresAt: clock().Add(user.PasswordResetTokenLifetime),
	}); err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	var preResetVersion int64
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM user_login_identities WHERE user_id = 'user-a' AND kind = 'password'`).Scan(&preResetVersion); err != nil {
		t.Fatalf("read pre-reset updated_at: %v", err)
	}

	// The reset commits in the same millisecond as the existing credential
	// version, which is the exact same-millisecond gap the old equality fence
	// left open.
	if _, err := store.ConsumeTokenAndSetPassword(ctx, "reset-hash", "new-hash", time.UnixMilli(preResetVersion)); err != nil {
		t.Fatalf("set password: %v", err)
	}

	var postResetVersion int64
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM user_login_identities WHERE user_id = 'user-a' AND kind = 'password'`).Scan(&postResetVersion); err != nil {
		t.Fatalf("read post-reset updated_at: %v", err)
	}
	if postResetVersion <= preResetVersion {
		t.Fatalf("post-reset credential version = %d, want > %d (strictly monotonic)", postResetVersion, preResetVersion)
	}

	userStore := newUserStore(db, func() time.Time { return *now })
	_, err = userStore.CreateSession(ctx, user.CreateSessionInput{
		UserID:            "user-a",
		TokenHash:         user.HashSessionToken("stale-login"),
		ExpiresAt:         clock().Add(user.SessionLifetime),
		FenceCredential:   true,
		CredentialVersion: preResetVersion,
	})
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("post-reset stale-credential session error = %v, want ErrInvalidCredentials", err)
	}
}

func TestContactChannelCreateTokenMapsPrimaryKeyCollisionToConflict(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, _ := newContactChannelStoreTestClock()
	seedContactChannelUser(t, ctx, db, "user-a", "ana")

	channel, err := store.UpsertUnverifiedEmail(ctx, "user-a", "ana@example.com", clock())
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}

	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: "dup-token-id", ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: "hash-a", CreatedAt: clock(), ExpiresAt: clock().Add(user.EmailVerificationTokenLifetime),
	}); err != nil {
		t.Fatalf("create first token: %v", err)
	}

	err = store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: "dup-token-id", ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeVerify,
		TokenHash: "hash-b", CreatedAt: clock(), ExpiresAt: clock().Add(user.EmailVerificationTokenLifetime),
	})
	if !errors.Is(err, user.ErrContactChannelTokenConflict) {
		t.Fatalf("duplicate-id token error = %v, want ErrContactChannelTokenConflict", err)
	}
}

func TestContactChannelSetPasswordUpgradesLocalOIDCIdentity(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := NewContactChannelStore(db)
	clock, _ := newContactChannelStoreTestClock()

	insertAuthorStoreUser(t, ctx, db, "user-o", "author-o", "Octavio")
	if _, err := db.ExecContext(ctx, `INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at) VALUES ('identity-o', 'user-o', 'oidc', 'local', 'octavio', NULL, 0, 0)`); err != nil {
		t.Fatalf("insert local identity: %v", err)
	}

	channel, err := store.UpsertUnverifiedEmail(ctx, "user-o", "octavio@example.com", clock())
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}
	markChannelVerified(t, ctx, db, channel.ID, clock())

	resetID, _ := user.NewContactChannelTokenID()
	if err := store.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID: resetID, ChannelID: channel.ID, Purpose: user.ContactChannelTokenPurposeReset,
		TokenHash: "reset-hash-o", CreatedAt: clock(), ExpiresAt: clock().Add(user.PasswordResetTokenLifetime),
	}); err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	if _, err := store.ConsumeTokenAndSetPassword(ctx, "reset-hash-o", "new-hash", clock()); err != nil {
		t.Fatalf("set password: %v", err)
	}

	var identityID, kind, identifier, secretHash string
	if err := db.QueryRowContext(ctx, `SELECT id, kind, normalized_identifier, secret_hash FROM user_login_identities WHERE user_id = 'user-o'`).Scan(&identityID, &kind, &identifier, &secretHash); err != nil {
		t.Fatalf("read upgraded local identity: %v", err)
	}
	if identityID != "identity-o" {
		t.Fatalf("identity id = %q, want identity-o", identityID)
	}
	if kind != string(user.LoginIdentityKindPassword) {
		t.Fatalf("identity kind = %q, want password", kind)
	}
	if identifier != "octavio" {
		t.Fatalf("normalized identifier = %q, want octavio", identifier)
	}
	if secretHash != "new-hash" {
		t.Fatalf("secret hash = %q, want new-hash", secretHash)
	}

	login, err := newUserStore(db, clock).FindPasswordLogin(ctx, "octavio")
	if err != nil {
		t.Fatalf("find password login: %v", err)
	}
	if login.User.ID != "user-o" {
		t.Fatalf("password login user id = %q, want user-o", login.User.ID)
	}
	if login.SecretHash != "new-hash" {
		t.Fatalf("password login secret hash = %q, want new-hash", login.SecretHash)
	}
}

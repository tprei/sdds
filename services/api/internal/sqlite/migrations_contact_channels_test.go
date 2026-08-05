package sqlite

import (
	"context"
	"testing"
)

func TestContactChannelMigrationAllowsManyUnverifiedHoldersOfOneAddress(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, "user-a", "author-a", "Ana")
	insertAuthorStoreUser(t, ctx, db, "user-b", "author-b", "Bruno")

	for _, userID := range []string{"user-a", "user-b"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES (?, ?, 'email', 'shared@example.com', NULL, NULL, 0, 0)`, userID+"-ch", userID); err != nil {
			t.Fatalf("insert unverified channel for %s: %v", userID, err)
		}
	}
}

func TestContactChannelMigrationRejectsTwoVerifiedHoldersOfOneAddress(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, "user-a", "author-a", "Ana")
	insertAuthorStoreUser(t, ctx, db, "user-b", "author-b", "Bruno")

	if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES ('ch-a', 'user-a', 'email', 'shared@example.com', 1, 'token', 0, 0)`); err != nil {
		t.Fatalf("insert first verified channel: %v", err)
	}

	_, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES ('ch-b', 'user-b', 'email', 'shared@example.com', 2, 'token', 0, 0)`)
	if err == nil {
		t.Fatal("second verified channel for the same address was accepted; want a unique-constraint failure")
	}
}

func TestContactChannelMigrationCascadesOnUserDelete(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES ('user-a', 'active', 0, 0)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at) VALUES ('ch-a', 'user-a', 'email', 'ana@example.com', NULL, NULL, 0, 0)`); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_contact_channel_tokens (id, channel_id, purpose, token_hash, created_at, expires_at, consumed_at) VALUES ('tok-a', 'ch-a', 'verify', 'hash-a', 0, 1, NULL)`); err != nil {
		t.Fatalf("insert token: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = 'user-a'`); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	channelCount := countRows(t, ctx, db, `SELECT COUNT(*) FROM user_contact_channels WHERE user_id = 'user-a'`)
	if channelCount != 0 {
		t.Fatalf("contact channels after user delete = %d, want 0", channelCount)
	}
	tokenCount := countRows(t, ctx, db, `SELECT COUNT(*) FROM user_contact_channel_tokens WHERE channel_id = 'ch-a'`)
	if tokenCount != 0 {
		t.Fatalf("contact channel tokens after user delete = %d, want 0", tokenCount)
	}
}

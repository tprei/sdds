package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	authordomain "github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	findOIDCIdentitySQL = `
		SELECT users.id, users.state
		FROM user_login_identities
		JOIN users ON users.id = user_login_identities.user_id
		WHERE user_login_identities.kind = ?
			AND user_login_identities.provider = ?
			AND user_login_identities.normalized_identifier = ?
	`
	findVerifiedEmailUserSQL = `
		SELECT user_id
		FROM user_contact_channels
		WHERE channel = ? AND normalized_value = ? AND verified_at IS NOT NULL
	`
	insertOIDCLoginIdentitySQL = `
		INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
	`
	insertOIDCContactChannelSQL = `
		INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	listLoginIdentitiesSQL = `
		SELECT id, kind, provider
		FROM user_login_identities
		WHERE user_id = ? AND NOT (provider = ? AND kind = ?)
		ORDER BY created_at, id
	`
	findLoginIdentityForUserSQL = `
		SELECT kind, provider
		FROM user_login_identities
		WHERE id = ? AND user_id = ?
	`
	countLoginIdentitiesSQL = `
		SELECT COUNT(*)
		FROM user_login_identities
		WHERE user_id = ? AND NOT (provider = ? AND kind = ?)
	`
	deleteLoginIdentitySQL = `
		DELETE FROM user_login_identities
		WHERE id = ? AND user_id = ?
	`
	stripPasswordCredentialSQL = `
		UPDATE user_login_identities
		SET kind = ?, secret_hash = NULL, updated_at = MAX(updated_at + 1, ?)
		WHERE id = ? AND user_id = ?
	`
)

var _ user.LoginIdentityStore = (*UserStore)(nil)

func (store *UserStore) ResolveOIDCIdentity(ctx context.Context, input user.ResolveOIDCIdentityInput) (current user.CurrentSession, err error) {
	now := normalizeTime(store.clock())
	expiresAt := normalizeTime(input.ExpiresAt)
	if !now.Before(expiresAt) {
		return user.CurrentSession{}, user.ErrSessionExpired
	}

	createdUserID, err := user.NewUserID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create oidc user id: %w", err)
	}
	authorID, err := authordomain.NewAuthorID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create oidc author id: %w", err)
	}
	localIdentityID, err := user.NewLoginIdentityID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create oidc local identity id: %w", err)
	}
	providerIdentityID, err := user.NewLoginIdentityID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create oidc provider identity id: %w", err)
	}
	sessionID, err := user.NewSessionID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create oidc session id: %w", err)
	}
	contactChannelID := user.ContactChannelID("")
	if input.Email != "" {
		contactChannelID, err = user.NewContactChannelID()
		if err != nil {
			return user.CurrentSession{}, fmt.Errorf("create oidc contact channel id: %w", err)
		}
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("begin resolve oidc identity: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback resolve oidc identity: %w", rollbackErr)
		}
	}()

	var existingUserID string
	var existingState string
	identityErr := tx.QueryRowContext(ctx, findOIDCIdentitySQL,
		user.LoginIdentityKindOIDC, input.Provider, input.Subject,
	).Scan(&existingUserID, &existingState)
	if identityErr == nil {
		if user.UserState(existingState) != user.UserStateActive {
			return user.CurrentSession{}, user.ErrUserDisabled
		}
		if err := insertOIDCSession(ctx, tx, sessionID, user.UserID(existingUserID), input.TokenHash, now, expiresAt); err != nil {
			return user.CurrentSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return user.CurrentSession{}, fmt.Errorf("commit oidc sign in: %w", err)
		}
		current, err = store.FindCurrentSession(ctx, input.TokenHash, now)
		if err != nil {
			return user.CurrentSession{}, fmt.Errorf("load oidc sign in session: %w", err)
		}
		return current, nil
	}
	if !errors.Is(identityErr, sql.ErrNoRows) {
		return user.CurrentSession{}, fmt.Errorf("find oidc identity: %w", identityErr)
	}

	if input.EmailVerified && input.Email != "" && user.ProviderOwnsEmailNamespace(input.Provider) {
		var linkedUserID string
		emailErr := tx.QueryRowContext(ctx, findVerifiedEmailUserSQL, user.ContactChannelEmail, input.Email).Scan(&linkedUserID)
		if emailErr == nil {
			if err := insertOIDCIdentity(ctx, tx, providerIdentityID, user.UserID(linkedUserID), input.Provider, input.Subject, now); err != nil {
				return user.CurrentSession{}, err
			}
			if err := insertOIDCSession(ctx, tx, sessionID, user.UserID(linkedUserID), input.TokenHash, now, expiresAt); err != nil {
				return user.CurrentSession{}, err
			}
			if err := tx.Commit(); err != nil {
				return user.CurrentSession{}, fmt.Errorf("commit linked oidc identity: %w", err)
			}
			current, err = store.FindCurrentSession(ctx, input.TokenHash, now)
			if err != nil {
				return user.CurrentSession{}, fmt.Errorf("load linked oidc session: %w", err)
			}
			return current, nil
		}
		if !errors.Is(emailErr, sql.ErrNoRows) {
			return user.CurrentSession{}, fmt.Errorf("find verified oidc email: %w", emailErr)
		}
	}

	if input.Username == "" {
		return user.CurrentSession{}, user.ErrUsernameRequired
	}

	displayName := input.DisplayName
	if displayName == "" {
		displayName = input.Username
	}
	displayName = truncateRunes(displayName, 60)
	createdUser := user.User{ID: createdUserID, State: user.UserStateActive, CreatedAt: now, UpdatedAt: now}
	createdAuthor := user.Author{
		ID:          authorID,
		UserID:      createdUserID,
		DisplayName: displayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := tx.ExecContext(ctx, insertUserSQL, createdUser.ID, createdUser.State, unixMillis(now), unixMillis(now)); err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert oidc user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertAuthorSQL, createdAuthor.ID, createdAuthor.UserID, createdAuthor.DisplayName, unixMillis(now), unixMillis(now)); err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert oidc author: %w", err)
	}
	if err := insertOIDCIdentity(ctx, tx, localIdentityID, createdUserID, user.LoginIdentityProviderLocal, input.Username, now); err != nil {
		if isUniqueConstraintError(err) {
			return user.CurrentSession{}, user.ErrUsernameTaken
		}
		return user.CurrentSession{}, err
	}
	if err := insertOIDCIdentity(ctx, tx, providerIdentityID, createdUserID, input.Provider, input.Subject, now); err != nil {
		return user.CurrentSession{}, err
	}
	if input.Email != "" {
		var verifiedAt any
		var verifiedVia any
		if input.EmailVerified {
			verifiedAt = unixMillis(now)
			verifiedVia = input.Provider
		}
		if _, err := tx.ExecContext(ctx, insertOIDCContactChannelSQL,
			contactChannelID, createdUserID, user.ContactChannelEmail, input.Email,
			verifiedAt, verifiedVia, unixMillis(now), unixMillis(now),
		); err != nil {
			return user.CurrentSession{}, fmt.Errorf("insert oidc contact channel: %w", err)
		}
	}
	if err := insertOIDCSession(ctx, tx, sessionID, createdUserID, input.TokenHash, now, expiresAt); err != nil {
		return user.CurrentSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return user.CurrentSession{}, fmt.Errorf("commit created oidc identity: %w", err)
	}
	current, err = store.FindCurrentSession(ctx, input.TokenHash, now)
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("load created oidc session: %w", err)
	}
	return current, nil
}

func insertOIDCIdentity(ctx context.Context, tx *sql.Tx, identityID user.LoginIdentityID, userID user.UserID, provider string, identifier string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, insertOIDCLoginIdentitySQL,
		identityID, userID, user.LoginIdentityKindOIDC, provider, identifier, unixMillis(now), unixMillis(now),
	); err != nil {
		return fmt.Errorf("insert oidc login identity: %w", err)
	}
	return nil
}

func insertOIDCSession(ctx context.Context, tx *sql.Tx, sessionID user.SessionID, userID user.UserID, tokenHash string, now time.Time, expiresAt time.Time) error {
	result, err := tx.ExecContext(ctx, insertSessionForActiveUserSQL,
		sessionID, tokenHash, unixMillis(now), unixMillis(expiresAt), userID, user.UserStateActive,
	)
	if err != nil {
		return fmt.Errorf("insert oidc session: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read inserted oidc session count: %w", err)
	}
	if inserted == 0 {
		return user.ErrUserDisabled
	}
	return nil
}

func truncateRunes(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

func (store *UserStore) ListLoginIdentities(ctx context.Context, userID user.UserID) (identities []user.LoginIdentitySummary, err error) {
	rows, err := store.db.QueryContext(ctx, listLoginIdentitiesSQL,
		userID, user.LoginIdentityProviderLocal, user.LoginIdentityKindOIDC,
	)
	if err != nil {
		return nil, fmt.Errorf("list login identities: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close login identity rows: %w", closeErr)
		}
	}()

	identities = make([]user.LoginIdentitySummary, 0)
	for rows.Next() {
		var identity user.LoginIdentitySummary
		var identityID string
		var kind string
		if err := rows.Scan(&identityID, &kind, &identity.Provider); err != nil {
			return nil, fmt.Errorf("scan login identity: %w", err)
		}
		identity.ID = user.LoginIdentityID(identityID)
		identity.Kind = user.LoginIdentityKind(kind)
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read login identities: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close login identity rows: %w", err)
	}
	return identities, nil
}

// DeleteLoginIdentity disconnects one sign-in method. Provider identities are
// deleted outright; the password identity is stripped to a username-only row
// (kind oidc, no secret) because the local row also carries the username. The
// username-only row itself is not a sign-in method and reports not-found, as
// does any identity owned by another user. The last remaining method is
// refused.
func (store *UserStore) DeleteLoginIdentity(ctx context.Context, userID user.UserID, identityID user.LoginIdentityID) (err error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete login identity: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback delete login identity: %w", rollbackErr)
		}
	}()

	var kind string
	var provider string
	lookupErr := tx.QueryRowContext(ctx, findLoginIdentityForUserSQL, identityID, userID).Scan(&kind, &provider)
	if errors.Is(lookupErr, sql.ErrNoRows) {
		return user.ErrLoginIdentityNotFound
	}
	if lookupErr != nil {
		return fmt.Errorf("find login identity: %w", lookupErr)
	}
	if provider == user.LoginIdentityProviderLocal && kind == string(user.LoginIdentityKindOIDC) {
		return user.ErrLoginIdentityNotFound
	}

	var count int
	if err := tx.QueryRowContext(ctx, countLoginIdentitiesSQL,
		userID, user.LoginIdentityProviderLocal, user.LoginIdentityKindOIDC,
	).Scan(&count); err != nil {
		return fmt.Errorf("count login identities: %w", err)
	}
	if count == 1 {
		return user.ErrLastLoginIdentity
	}
	if provider == user.LoginIdentityProviderLocal && kind == string(user.LoginIdentityKindPassword) {
		// The local row is the account's username row; disconnecting the
		// password removes the credential and keeps the row, so the username
		// survives and a later password reset can restore the credential. The
		// updated_at bump advances the credential version, so an in-flight
		// password login fails its session fence.
		now := normalizeTime(store.clock())
		if _, err := tx.ExecContext(ctx, stripPasswordCredentialSQL, user.LoginIdentityKindOIDC, unixMillis(now), identityID, userID); err != nil {
			return fmt.Errorf("strip password credential: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, deleteLoginIdentitySQL, identityID, userID); err != nil {
		return fmt.Errorf("delete login identity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete login identity: %w", err)
	}
	return nil
}

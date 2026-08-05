package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	insertContactChannelSQL = `
		INSERT INTO user_contact_channels (id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, NULL, ?, ?)
	`
	updateContactChannelTouchedSQL = `
		UPDATE user_contact_channels SET updated_at = ? WHERE id = ?
	`
	findContactChannelByUserAndValueSQL = `
		SELECT id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at
		FROM user_contact_channels
		WHERE user_id = ? AND channel = ? AND normalized_value = ?
	`
	findEmailForUserSQL = `
		SELECT id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at
		FROM user_contact_channels
		WHERE user_id = ? AND channel = 'email'
		ORDER BY verified_at IS NULL, updated_at DESC
		LIMIT 1
	`
	findPendingEmailForUserSQL = `
		SELECT id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at
		FROM user_contact_channels
		WHERE user_id = ? AND channel = 'email' AND verified_at IS NULL
		ORDER BY updated_at DESC
		LIMIT 1
	`
	findVerifiedEmailSQL = `
		SELECT id, user_id, channel, normalized_value, verified_at, verified_via, created_at, updated_at
		FROM user_contact_channels
		WHERE channel = 'email' AND normalized_value = ? AND verified_at IS NOT NULL
	`
	findContactChannelByTokenSQL = `
		SELECT user_contact_channels.id, user_contact_channels.user_id, user_contact_channels.channel,
			user_contact_channels.normalized_value, user_contact_channels.verified_at, user_contact_channels.verified_via,
			user_contact_channels.created_at, user_contact_channels.updated_at
		FROM user_contact_channel_tokens
		JOIN user_contact_channels ON user_contact_channels.id = user_contact_channel_tokens.channel_id
		WHERE user_contact_channel_tokens.token_hash = ? AND user_contact_channel_tokens.purpose = ?
	`
	findVerifiedEmailValueForUserSQL = `
		SELECT normalized_value FROM user_contact_channels
		WHERE user_id = ? AND channel = 'email' AND verified_at IS NOT NULL
		LIMIT 1
	`
	deletePendingEmailTokensSQL = `
		DELETE FROM user_contact_channel_tokens
		WHERE channel_id IN (
			SELECT id FROM user_contact_channels
			WHERE user_id = ? AND channel = 'email' AND verified_at IS NULL
		)
	`
	deletePendingEmailsSQL = `
		DELETE FROM user_contact_channels
		WHERE user_id = ? AND channel = 'email' AND verified_at IS NULL
	`
	deleteOtherPendingEmailsSQL = `
		DELETE FROM user_contact_channels
		WHERE user_id = ? AND channel = 'email' AND verified_at IS NULL AND id != ?
	`
	insertContactChannelTokenSQL = `
		INSERT INTO user_contact_channel_tokens (id, channel_id, purpose, token_hash, created_at, expires_at, consumed_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL)
	`
	consumeContactChannelTokenSQL = `
		UPDATE user_contact_channel_tokens
		SET consumed_at = ?
		WHERE token_hash = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?
	`
	markContactChannelVerifiedSQL = `
		UPDATE user_contact_channels
		SET verified_at = COALESCE(verified_at, ?), verified_via = COALESCE(verified_via, ?), updated_at = ?
		WHERE id = ?
	`
	updatePasswordIdentitySQL = `
		UPDATE user_login_identities
		SET secret_hash = ?, updated_at = MAX(updated_at + 1, ?)
		WHERE user_id = ? AND kind = ? AND provider = ?
	`
	insertPasswordIdentitySQL = `
		INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	findLoginIdentifierForUserSQL = `
		SELECT normalized_identifier
		FROM user_login_identities
		WHERE user_id = ? AND kind = ? AND provider = ?
		LIMIT 1
	`
	revokeAllSessionsSQL = `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, ?)
		WHERE user_id = ? AND revoked_at IS NULL
	`
	consumeAllAccountTokensSQL = `
		UPDATE user_contact_channel_tokens
		SET consumed_at = COALESCE(consumed_at, ?)
		WHERE consumed_at IS NULL
			AND channel_id IN (SELECT id FROM user_contact_channels WHERE user_id = ?)
	`
)

var _ user.ContactChannelStore = (*ContactChannelStore)(nil)

type ContactChannelStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewContactChannelStore(db *sql.DB) *ContactChannelStore {
	return &ContactChannelStore{db: db, clock: time.Now}
}

func (store *ContactChannelStore) UpsertUnverifiedEmail(ctx context.Context, userID user.UserID, normalizedValue string, now time.Time) (record user.ContactChannelRecord, err error) {
	now = normalizeTime(now)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("begin upsert email: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback upsert email: %w", rollbackErr)
		}
	}()

	existing, found, err := queryContactChannel(ctx, tx.QueryRowContext(ctx, findContactChannelByUserAndValueSQL, userID, user.ContactChannelEmail, normalizedValue))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
	}
	if found {
		if _, err := tx.ExecContext(ctx, updateContactChannelTouchedSQL, unixMillis(now), existing.ID); err != nil {
			return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
		}
		existing.UpdatedAt = now
		if err := tx.Commit(); err != nil {
			return user.ContactChannelRecord{}, fmt.Errorf("commit upsert email: %w", err)
		}
		return existing, nil
	}

	if _, err := tx.ExecContext(ctx, deletePendingEmailTokensSQL, userID); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deletePendingEmailsSQL, userID); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
	}

	channelID, err := user.NewContactChannelID()
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertContactChannelSQL,
		channelID, userID, user.ContactChannelEmail, normalizedValue, unixMillis(now), unixMillis(now),
	); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("upsert email: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("commit upsert email: %w", err)
	}
	return user.ContactChannelRecord{
		ID:        channelID,
		UserID:    userID,
		Channel:   user.ContactChannelEmail,
		Value:     normalizedValue,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (store *ContactChannelStore) FindEmailForUser(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error) {
	record, found, err := queryContactChannel(ctx, store.db.QueryRowContext(ctx, findEmailForUserSQL, userID))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("find email for user: %w", err)
	}
	if !found {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return record, nil
}

func (store *ContactChannelStore) FindPendingEmailForUser(ctx context.Context, userID user.UserID) (user.ContactChannelRecord, error) {
	record, found, err := queryContactChannel(ctx, store.db.QueryRowContext(ctx, findPendingEmailForUserSQL, userID))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("find pending email: %w", err)
	}
	if !found {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return record, nil
}

func (store *ContactChannelStore) FindVerifiedEmail(ctx context.Context, normalizedValue string) (user.ContactChannelRecord, error) {
	record, found, err := queryContactChannel(ctx, store.db.QueryRowContext(ctx, findVerifiedEmailSQL, normalizedValue))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("find verified email: %w", err)
	}
	if !found {
		return user.ContactChannelRecord{}, user.ErrContactChannelNotFound
	}
	return record, nil
}

func (store *ContactChannelStore) CreateToken(ctx context.Context, input user.CreateContactChannelTokenInput) error {
	_, err := store.db.ExecContext(ctx, insertContactChannelTokenSQL,
		input.ID, input.ChannelID, input.Purpose, input.TokenHash,
		unixMillis(input.CreatedAt), unixMillis(input.ExpiresAt),
	)
	if err != nil {
		if isConstraintConflictError(err) {
			return user.ErrContactChannelTokenConflict
		}
		return fmt.Errorf("create contact channel token: %w", err)
	}
	return nil
}

// ConsumeTokenAndMarkVerified consumes a verification token and marks its
// channel verified in one transaction. Any failure rolls back, leaving the
// token unconsumed and retryable.
func (store *ContactChannelStore) ConsumeTokenAndMarkVerified(ctx context.Context, tokenHash string, verifiedVia string, now time.Time) (record user.ContactChannelRecord, err error) {
	now = normalizeTime(now)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("begin verify email: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback verify email: %w", rollbackErr)
		}
	}()

	result, err := tx.ExecContext(ctx, consumeContactChannelTokenSQL, unixMillis(now), tokenHash, user.ContactChannelTokenPurposeVerify, unixMillis(now))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("verify email: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("verify email: %w", err)
	}
	if rows == 0 {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}

	channel, found, err := queryContactChannel(ctx, tx.QueryRowContext(ctx, findContactChannelByTokenSQL, tokenHash, user.ContactChannelTokenPurposeVerify))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("verify email: %w", err)
	}
	if !found {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}

	if _, err := tx.ExecContext(ctx, markContactChannelVerifiedSQL, unixMillis(now), verifiedVia, unixMillis(now), channel.ID); err != nil {
		if isUniqueConstraintError(err) {
			return user.ContactChannelRecord{}, user.ErrContactChannelAlreadyVerified
		}
		return user.ContactChannelRecord{}, fmt.Errorf("verify email: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteOtherPendingEmailsSQL, channel.UserID, channel.ID); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("verify email: %w", err)
	}
	channel.VerifiedAt = &now
	channel.VerifiedVia = verifiedVia
	if err := tx.Commit(); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("commit verify email: %w", err)
	}
	return channel, nil
}

// ConsumeTokenAndSetPassword consumes a reset token, sets the password
// credential, revokes every session, and consumes every outstanding account
// token, all in one transaction. Any failure rolls back, leaving the token
// unconsumed and retryable.
func (store *ContactChannelStore) ConsumeTokenAndSetPassword(ctx context.Context, tokenHash string, secretHash string, now time.Time) (record user.ContactChannelRecord, err error) {
	now = normalizeTime(now)
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("begin set password: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback set password: %w", rollbackErr)
		}
	}()

	result, err := tx.ExecContext(ctx, consumeContactChannelTokenSQL, unixMillis(now), tokenHash, user.ContactChannelTokenPurposeReset, unixMillis(now))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	if rows == 0 {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}

	channel, found, err := queryContactChannel(ctx, tx.QueryRowContext(ctx, findContactChannelByTokenSQL, tokenHash, user.ContactChannelTokenPurposeReset))
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	if !found {
		return user.ContactChannelRecord{}, user.ErrContactChannelTokenInvalid
	}

	updateResult, err := tx.ExecContext(ctx, updatePasswordIdentitySQL, secretHash, unixMillis(now), channel.UserID, user.LoginIdentityKindPassword, user.LoginIdentityProviderLocal)
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	updated, err := updateResult.RowsAffected()
	if err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	if updated == 0 {
		identifier, err := loginIdentifierForUser(ctx, tx, channel.UserID)
		if err != nil {
			return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
		}
		identityID, err := user.NewLoginIdentityID()
		if err != nil {
			return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
		}
		if _, err := tx.ExecContext(ctx, insertPasswordIdentitySQL,
			identityID, channel.UserID, user.LoginIdentityKindPassword, user.LoginIdentityProviderLocal, identifier, secretHash, unixMillis(now), unixMillis(now),
		); err != nil {
			return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, revokeAllSessionsSQL, unixMillis(now), channel.UserID); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, consumeAllAccountTokensSQL, unixMillis(now), channel.UserID); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("set password: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return user.ContactChannelRecord{}, fmt.Errorf("commit set password: %w", err)
	}
	return channel, nil
}

func loginIdentifierForUser(ctx context.Context, tx *sql.Tx, userID user.UserID) (string, error) {
	var identifier string
	err := tx.QueryRowContext(ctx, findLoginIdentifierForUserSQL, userID, user.LoginIdentityKindPassword, user.LoginIdentityProviderLocal).Scan(&identifier)
	if err == nil {
		return identifier, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		var value string
		if scanErr := tx.QueryRowContext(ctx, findVerifiedEmailValueForUserSQL, userID).Scan(&value); scanErr != nil {
			return "", scanErr
		}
		return value, nil
	}
	return "", err
}

// queryContactChannel scans one contact-channel row. found is false when the row
// is absent (sql.ErrNoRows) so callers can map it to a domain sentinel.
func queryContactChannel(_ context.Context, row *sql.Row) (user.ContactChannelRecord, bool, error) {
	var (
		id          string
		userID      string
		channel     string
		value       string
		verifiedAt  sql.NullInt64
		verifiedVia sql.NullString
		createdAt   int64
		updatedAt   int64
	)
	if err := row.Scan(&id, &userID, &channel, &value, &verifiedAt, &verifiedVia, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.ContactChannelRecord{}, false, nil
		}
		return user.ContactChannelRecord{}, false, err
	}
	record := user.ContactChannelRecord{
		ID:        user.ContactChannelID(id),
		UserID:    user.UserID(userID),
		Channel:   user.ContactChannel(channel),
		Value:     value,
		CreatedAt: timeFromUnixMillis(createdAt),
		UpdatedAt: timeFromUnixMillis(updatedAt),
	}
	if verifiedAt.Valid {
		verified := timeFromUnixMillis(verifiedAt.Int64)
		record.VerifiedAt = &verified
	}
	if verifiedVia.Valid {
		record.VerifiedVia = verifiedVia.String
	}
	return record, true, nil
}

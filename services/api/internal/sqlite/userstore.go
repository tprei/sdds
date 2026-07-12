package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	authordomain "github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/user"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	insertUserSQL = `
		INSERT INTO users (id, state, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`
	insertAuthorSQL = `
		INSERT INTO authors (id, user_id, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	insertPasswordLoginIdentitySQL = `
		INSERT INTO user_login_identities (id, user_id, kind, provider, normalized_identifier, secret_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	insertSessionSQL = `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, NULL)
	`
	insertSessionForActiveUserSQL = `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at, revoked_at)
		SELECT ?, users.id, ?, ?, ?, NULL
		FROM users
		WHERE users.id = ?
			AND users.state = ?
	`
	findPasswordLoginSQL = `
		SELECT
			users.id,
			users.state,
			users.created_at,
			users.updated_at,
			authors.id,
			authors.display_name,
			authors.created_at,
			authors.updated_at,
			user_login_identities.normalized_identifier,
			user_login_identities.secret_hash
		FROM user_login_identities
		JOIN users ON users.id = user_login_identities.user_id
		JOIN authors ON authors.user_id = users.id
		WHERE user_login_identities.kind = ?
			AND user_login_identities.provider = ?
			AND user_login_identities.normalized_identifier = ?
	`
	findCurrentSessionSQL = `
		SELECT
			sessions.id,
			sessions.user_id,
			sessions.token_hash,
			sessions.created_at,
			sessions.expires_at,
			sessions.revoked_at,
			users.state,
			users.created_at,
			users.updated_at,
			authors.id,
			authors.display_name,
			authors.created_at,
			authors.updated_at,
			user_login_identities.normalized_identifier
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		JOIN authors ON authors.user_id = users.id
		LEFT JOIN user_login_identities ON user_login_identities.user_id = users.id
			AND user_login_identities.kind = ?
			AND user_login_identities.provider = ?
		WHERE sessions.token_hash = ?
	`
	revokeSessionSQL = `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, ?)
		WHERE id = ?
	`
	findAuthorByUserIDSQL = `
		SELECT id, user_id, display_name, created_at, updated_at
		FROM authors
		WHERE user_id = ?
	`
)

var _ user.Store = (*UserStore)(nil)

type UserStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewUserStore(db *sql.DB) *UserStore {
	return newUserStore(db, time.Now)
}

func newUserStore(db *sql.DB, clock func() time.Time) *UserStore {
	return &UserStore{db: db, clock: clock}
}

func (store *UserStore) CreatePasswordUser(ctx context.Context, input user.CreatePasswordUserInput) (current user.CurrentSession, err error) {
	now := normalizeTime(store.clock())
	if !now.Before(input.ExpiresAt) {
		return user.CurrentSession{}, user.ErrSessionExpired
	}
	createdUser, author, loginIdentityID, session, err := newPasswordUserRecords(input, now)
	if err != nil {
		return user.CurrentSession{}, err
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("begin create password user: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback create password user: %w", rollbackErr)
		}
	}()

	if _, err := tx.ExecContext(ctx, insertUserSQL, createdUser.ID, createdUser.State, unixMillis(createdUser.CreatedAt), unixMillis(createdUser.UpdatedAt)); err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertAuthorSQL, author.ID, author.UserID, author.DisplayName, unixMillis(author.CreatedAt), unixMillis(author.UpdatedAt)); err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert author: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		insertPasswordLoginIdentitySQL,
		loginIdentityID,
		createdUser.ID,
		user.LoginIdentityKindPassword,
		user.LoginIdentityProviderLocal,
		input.Username,
		input.SecretHash,
		unixMillis(now),
		unixMillis(now),
	); err != nil {
		if isUniqueConstraintError(err) {
			return user.CurrentSession{}, user.ErrUsernameTaken
		}
		return user.CurrentSession{}, fmt.Errorf("insert password login identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insertSessionSQL, session.ID, session.UserID, session.TokenHash, unixMillis(session.CreatedAt), unixMillis(session.ExpiresAt)); err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return user.CurrentSession{}, fmt.Errorf("commit create password user: %w", err)
	}

	return user.CurrentSession{Session: session, User: createdUser, Author: author, Username: input.Username}, nil
}

func (store *UserStore) FindPasswordLogin(ctx context.Context, normalizedUsername string) (user.PasswordLogin, error) {
	row := store.db.QueryRowContext(
		ctx,
		findPasswordLoginSQL,
		user.LoginIdentityKindPassword,
		user.LoginIdentityProviderLocal,
		normalizedUsername,
	)
	login, err := scanPasswordLogin(row)
	if err == nil {
		return login, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return user.PasswordLogin{}, user.ErrInvalidCredentials
	}
	return user.PasswordLogin{}, fmt.Errorf("find password login: %w", err)
}

func (store *UserStore) CreateSession(ctx context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
	now := normalizeTime(store.clock())
	if !now.Before(input.ExpiresAt) {
		return user.CurrentSession{}, user.ErrSessionExpired
	}
	sessionID, err := user.NewSessionID()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("create session id: %w", err)
	}

	result, err := store.db.ExecContext(
		ctx,
		insertSessionForActiveUserSQL,
		sessionID,
		input.TokenHash,
		unixMillis(now),
		unixMillis(input.ExpiresAt),
		input.UserID,
		user.UserStateActive,
	)
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("insert session: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("read inserted session count: %w", err)
	}
	if inserted == 0 {
		return user.CurrentSession{}, user.ErrUserDisabled
	}

	current, err := store.FindCurrentSession(ctx, input.TokenHash, now)
	if err != nil {
		return user.CurrentSession{}, fmt.Errorf("load created session: %w", err)
	}
	return current, nil
}

func (store *UserStore) FindCurrentSession(ctx context.Context, tokenHash string, now time.Time) (user.CurrentSession, error) {
	row := store.db.QueryRowContext(
		ctx,
		findCurrentSessionSQL,
		user.LoginIdentityKindPassword,
		user.LoginIdentityProviderLocal,
		tokenHash,
	)
	current, err := scanCurrentSession(row)
	if err == nil {
		return rejectInactiveCurrentSession(current, normalizeTime(now))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return user.CurrentSession{}, user.ErrSessionNotFound
	}
	return user.CurrentSession{}, fmt.Errorf("find current session: %w", err)
}

func (store *UserStore) RevokeSession(ctx context.Context, sessionID user.SessionID, revokedAt time.Time) error {
	if _, err := store.db.ExecContext(ctx, revokeSessionSQL, unixMillis(normalizeTime(revokedAt)), sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (store *UserStore) FindAuthorByUserID(ctx context.Context, userID user.UserID) (user.Author, error) {
	var author user.Author
	var id string
	var scannedUserID string
	var createdAt int64
	var updatedAt int64
	err := store.db.QueryRowContext(ctx, findAuthorByUserIDSQL, userID).Scan(&id, &scannedUserID, &author.DisplayName, &createdAt, &updatedAt)
	if err == nil {
		author.ID = authordomain.AuthorID(id)
		author.UserID = user.UserID(scannedUserID)
		author.CreatedAt = timeFromUnixMillis(createdAt)
		author.UpdatedAt = timeFromUnixMillis(updatedAt)
		return author, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return user.Author{}, authordomain.ErrAuthorNotFound
	}
	return user.Author{}, fmt.Errorf("find author by user id: %w", err)
}

func newPasswordUserRecords(input user.CreatePasswordUserInput, now time.Time) (user.User, user.Author, user.LoginIdentityID, user.Session, error) {
	userID, err := user.NewUserID()
	if err != nil {
		return user.User{}, user.Author{}, "", user.Session{}, fmt.Errorf("create user id: %w", err)
	}
	authorID, err := authordomain.NewAuthorID()
	if err != nil {
		return user.User{}, user.Author{}, "", user.Session{}, fmt.Errorf("create author id: %w", err)
	}
	loginIdentityID, err := user.NewLoginIdentityID()
	if err != nil {
		return user.User{}, user.Author{}, "", user.Session{}, fmt.Errorf("create login identity id: %w", err)
	}
	sessionID, err := user.NewSessionID()
	if err != nil {
		return user.User{}, user.Author{}, "", user.Session{}, fmt.Errorf("create session id: %w", err)
	}

	createdUser := user.User{
		ID:        userID,
		State:     user.UserStateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	author := user.Author{
		ID:          authorID,
		UserID:      userID,
		DisplayName: input.DisplayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	session := user.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: input.TokenHash,
		CreatedAt: now,
		ExpiresAt: normalizeTime(input.ExpiresAt),
	}
	return createdUser, author, loginIdentityID, session, nil
}

func scanPasswordLogin(scan *sql.Row) (user.PasswordLogin, error) {
	var userID string
	var authorID string
	var userCreatedAt int64
	var userUpdatedAt int64
	var authorCreatedAt int64
	var authorUpdatedAt int64
	var state string
	var displayName string
	var username string
	var secretHash sql.NullString
	if err := scan.Scan(
		&userID,
		&state,
		&userCreatedAt,
		&userUpdatedAt,
		&authorID,
		&displayName,
		&authorCreatedAt,
		&authorUpdatedAt,
		&username,
		&secretHash,
	); err != nil {
		return user.PasswordLogin{}, err
	}
	if !secretHash.Valid {
		return user.PasswordLogin{}, user.ErrInvalidCredentials
	}

	currentUserID := user.UserID(userID)
	return user.PasswordLogin{
		User: user.User{
			ID:        currentUserID,
			State:     user.UserState(state),
			CreatedAt: timeFromUnixMillis(userCreatedAt),
			UpdatedAt: timeFromUnixMillis(userUpdatedAt),
		},
		Author: user.Author{
			ID:          authordomain.AuthorID(authorID),
			UserID:      currentUserID,
			DisplayName: displayName,
			CreatedAt:   timeFromUnixMillis(authorCreatedAt),
			UpdatedAt:   timeFromUnixMillis(authorUpdatedAt),
		},
		Username:   username,
		SecretHash: secretHash.String,
	}, nil
}

func scanCurrentSession(scan *sql.Row) (user.CurrentSession, error) {
	var sessionID string
	var sessionUserID string
	var tokenHash string
	var sessionCreatedAt int64
	var sessionExpiresAt int64
	var sessionRevokedAt sql.NullInt64
	var state string
	var userCreatedAt int64
	var userUpdatedAt int64
	var authorID string
	var displayName string
	var authorCreatedAt int64
	var authorUpdatedAt int64
	var username sql.NullString

	if err := scan.Scan(
		&sessionID,
		&sessionUserID,
		&tokenHash,
		&sessionCreatedAt,
		&sessionExpiresAt,
		&sessionRevokedAt,
		&state,
		&userCreatedAt,
		&userUpdatedAt,
		&authorID,
		&displayName,
		&authorCreatedAt,
		&authorUpdatedAt,
		&username,
	); err != nil {
		return user.CurrentSession{}, err
	}

	currentUserID := user.UserID(sessionUserID)
	session := user.Session{
		ID:        user.SessionID(sessionID),
		UserID:    currentUserID,
		TokenHash: tokenHash,
		CreatedAt: timeFromUnixMillis(sessionCreatedAt),
		ExpiresAt: timeFromUnixMillis(sessionExpiresAt),
	}
	if sessionRevokedAt.Valid {
		revokedAt := timeFromUnixMillis(sessionRevokedAt.Int64)
		session.RevokedAt = &revokedAt
	}
	current := user.CurrentSession{
		Session: session,
		User: user.User{
			ID:        currentUserID,
			State:     user.UserState(state),
			CreatedAt: timeFromUnixMillis(userCreatedAt),
			UpdatedAt: timeFromUnixMillis(userUpdatedAt),
		},
		Author: user.Author{
			ID:          authordomain.AuthorID(authorID),
			UserID:      currentUserID,
			DisplayName: displayName,
			CreatedAt:   timeFromUnixMillis(authorCreatedAt),
			UpdatedAt:   timeFromUnixMillis(authorUpdatedAt),
		},
	}
	if username.Valid {
		current.Username = username.String
	}
	return current, nil
}

func rejectInactiveCurrentSession(current user.CurrentSession, now time.Time) (user.CurrentSession, error) {
	if current.User.State != user.UserStateActive {
		return user.CurrentSession{}, user.ErrUserDisabled
	}
	if current.Session.RevokedAt != nil {
		return user.CurrentSession{}, user.ErrSessionRevoked
	}
	if !now.Before(current.Session.ExpiresAt) {
		return user.CurrentSession{}, user.ErrSessionExpired
	}
	return current, nil
}

func isUniqueConstraintError(err error) bool {
	var sqliteErr *modernsqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

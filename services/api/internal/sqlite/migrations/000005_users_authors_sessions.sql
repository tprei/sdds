CREATE TABLE users (
	id TEXT PRIMARY KEY,
	state TEXT NOT NULL CHECK (state IN ('active', 'disabled')),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE authors (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
	display_name TEXT NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 60),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE user_login_identities (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	kind TEXT NOT NULL CHECK (kind IN ('password', 'oidc')),
	provider TEXT NOT NULL CHECK (length(trim(provider)) > 0),
	normalized_identifier TEXT NOT NULL CHECK (length(trim(normalized_identifier)) > 0),
	secret_hash TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	CHECK (
		(kind = 'password' AND provider = 'local' AND secret_hash IS NOT NULL)
		OR (kind = 'oidc' AND secret_hash IS NULL)
	),
	UNIQUE(kind, provider, normalized_identifier)
);

CREATE INDEX user_login_identities_user_idx ON user_login_identities (user_id);
CREATE UNIQUE INDEX user_login_identities_one_password_provider_per_user_idx
	ON user_login_identities (user_id, kind, provider)
	WHERE kind = 'password';

CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	token_hash TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	revoked_at INTEGER
);

CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_active_expiry_idx ON sessions (expires_at, revoked_at);

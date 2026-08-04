CREATE TABLE user_contact_channels (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	channel TEXT NOT NULL CHECK (channel IN ('email', 'phone')),
	normalized_value TEXT NOT NULL CHECK (length(trim(normalized_value)) > 0),
	verified_at INTEGER,
	verified_via TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	CHECK ((verified_at IS NULL) = (verified_via IS NULL)),
	UNIQUE(user_id, channel, normalized_value)
);

CREATE INDEX user_contact_channels_user_idx ON user_contact_channels (user_id);

CREATE UNIQUE INDEX user_contact_channels_verified_value_idx
	ON user_contact_channels (channel, normalized_value)
	WHERE verified_at IS NOT NULL;

CREATE TABLE user_contact_channel_tokens (
	id TEXT PRIMARY KEY,
	channel_id TEXT NOT NULL REFERENCES user_contact_channels(id) ON DELETE CASCADE,
	purpose TEXT NOT NULL CHECK (purpose IN ('verify', 'reset')),
	token_hash TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	consumed_at INTEGER
);

CREATE INDEX user_contact_channel_tokens_channel_idx ON user_contact_channel_tokens (channel_id, purpose);

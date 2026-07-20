CREATE TABLE image_uploads (
	id TEXT NOT NULL PRIMARY KEY CHECK (typeof(id) = 'text' AND length(id) > 0),
	user_id TEXT NOT NULL REFERENCES users(id),
	storage_key TEXT NOT NULL UNIQUE CHECK (typeof(storage_key) = 'text' AND length(storage_key) > 0),
	upload_request_id TEXT NOT NULL CHECK (typeof(upload_request_id) = 'text' AND length(upload_request_id) BETWEEN 1 AND 128),
	state TEXT NOT NULL CHECK (typeof(state) = 'text' AND state IN ('pending', 'ready', 'consumed', 'deleting', 'expired')),
	consumed_note_id TEXT REFERENCES notes(id) ON DELETE SET NULL,
	content_type TEXT NOT NULL CHECK (typeof(content_type) = 'text' AND length(content_type) BETWEEN 7 AND 128 AND substr(content_type, 1, 6) = 'image/'),
	byte_size INTEGER NOT NULL CHECK (typeof(byte_size) = 'integer' AND byte_size > 0),
	width INTEGER NOT NULL CHECK (typeof(width) = 'integer' AND width > 0),
	height INTEGER NOT NULL CHECK (typeof(height) = 'integer' AND height > 0),
	sha256 TEXT NOT NULL CHECK (typeof(sha256) = 'text' AND length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
	created_at INTEGER NOT NULL CHECK (typeof(created_at) = 'integer'),
	updated_at INTEGER NOT NULL CHECK (typeof(updated_at) = 'integer'),
	write_lease_until INTEGER CHECK (write_lease_until IS NULL OR (typeof(write_lease_until) = 'integer' AND write_lease_until > updated_at)),
	expires_at INTEGER NOT NULL CHECK (typeof(expires_at) = 'integer' AND expires_at > created_at),
	request_retention_until INTEGER NOT NULL CHECK (typeof(request_retention_until) = 'integer' AND request_retention_until > expires_at),
	UNIQUE (user_id, upload_request_id),
	CHECK (state = 'consumed' OR consumed_note_id IS NULL)
);
CREATE INDEX image_uploads_cleanup_idx ON image_uploads (state, expires_at, request_retention_until);
CREATE INDEX image_uploads_user_idx ON image_uploads (user_id, created_at DESC);

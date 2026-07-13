CREATE TABLE note_images (
	id           TEXT NOT NULL PRIMARY KEY
		CHECK (typeof(id) = 'text' AND length(id) > 0),
	note_id      TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
	storage_key  TEXT NOT NULL UNIQUE
		CHECK (typeof(storage_key) = 'text' AND length(storage_key) > 0),
	content_type TEXT NOT NULL
		CHECK (typeof(content_type) = 'text' AND length(content_type) BETWEEN 7 AND 128 AND substr(content_type, 1, 6) = 'image/'),
	byte_size    INTEGER NOT NULL
		CHECK (typeof(byte_size) = 'integer' AND byte_size > 0),
	width        INTEGER NOT NULL
		CHECK (typeof(width) = 'integer' AND width > 0),
	height       INTEGER NOT NULL
		CHECK (typeof(height) = 'integer' AND height > 0),
	sha256       TEXT NOT NULL
		CHECK (typeof(sha256) = 'text' AND length(sha256) = 64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
	position     INTEGER NOT NULL
		CHECK (typeof(position) = 'integer' AND position >= 0),
	created_at   INTEGER NOT NULL
		CHECK (typeof(created_at) = 'integer'),
	updated_at   INTEGER NOT NULL
		CHECK (typeof(updated_at) = 'integer'),
	UNIQUE (note_id, position)
);

CREATE TABLE note_create_requests (
	user_id           TEXT NOT NULL REFERENCES users(id),
	client_request_id TEXT NOT NULL
		CHECK (typeof(client_request_id) = 'text' AND length(client_request_id) BETWEEN 1 AND 128),
	request_sha256    TEXT NOT NULL
		CHECK (typeof(request_sha256) = 'text' AND length(request_sha256) = 64 AND request_sha256 NOT GLOB '*[^0-9a-f]*'),
	note_id           TEXT NOT NULL UNIQUE REFERENCES notes(id) ON DELETE CASCADE,
	created_at        INTEGER NOT NULL
		CHECK (typeof(created_at) = 'integer'),
	PRIMARY KEY (user_id, client_request_id)
);

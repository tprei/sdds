CREATE TABLE note_useful_reactions (
	note_id   TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
	user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (note_id, user_id)
);

CREATE INDEX note_useful_reactions_user_idx ON note_useful_reactions (user_id);

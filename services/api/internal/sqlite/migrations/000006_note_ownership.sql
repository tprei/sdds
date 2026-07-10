DROP TABLE note_search;
DROP INDEX notes_recent_idx;
DROP INDEX notes_category_idx;
DROP INDEX notes_place_idx;

ALTER TABLE notes RENAME TO notes_legacy;

INSERT INTO users (id, state, created_at, updated_at)
SELECT '00000000-0000-7000-8000-000000000001', 'active', 0, 0
WHERE EXISTS (SELECT 1 FROM notes_legacy);

INSERT INTO authors (id, user_id, display_name, created_at, updated_at)
SELECT
	'00000000-0000-7000-8000-000000000002',
	'00000000-0000-7000-8000-000000000001',
	'sdds',
	0,
	0
WHERE EXISTS (SELECT 1 FROM notes_legacy);

CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 3 AND 120),
	body TEXT NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 4000),
	category_slug TEXT NOT NULL REFERENCES categories(slug),
	place_slug TEXT REFERENCES places(slug),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
SELECT
	id,
	'00000000-0000-7000-8000-000000000001',
	title,
	body,
	category_slug,
	place_slug,
	created_at,
	updated_at
FROM notes_legacy;

CREATE INDEX notes_recent_idx ON notes (created_at DESC, id DESC);
CREATE INDEX notes_category_idx ON notes (category_slug);
CREATE INDEX notes_place_idx ON notes (place_slug);
CREATE INDEX notes_user_idx ON notes (user_id);

CREATE VIRTUAL TABLE note_search USING fts5(
	note_id UNINDEXED,
	title,
	body,
	tokenize = 'unicode61 remove_diacritics 2'
);

INSERT INTO note_search (note_id, title, body)
SELECT id, title, body
FROM notes;

DROP TABLE notes_legacy;

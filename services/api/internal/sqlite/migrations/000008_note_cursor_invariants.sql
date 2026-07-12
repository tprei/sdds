DROP TABLE note_search;
DROP INDEX notes_recent_idx;
DROP INDEX notes_category_idx;
DROP INDEX notes_place_idx;
DROP INDEX notes_user_idx;
DROP INDEX notes_author_page_idx;

ALTER TABLE notes RENAME TO notes_legacy;

CREATE TABLE notes (
	id TEXT NOT NULL UNIQUE CHECK (typeof(id) = 'text'),
	user_id TEXT NOT NULL REFERENCES users(id),
	title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 3 AND 120),
	body TEXT NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 4000),
	category_slug TEXT NOT NULL REFERENCES categories(slug),
	place_slug TEXT REFERENCES places(slug),
	created_at INTEGER NOT NULL CHECK (typeof(created_at) = 'integer'),
	updated_at INTEGER NOT NULL CHECK (typeof(updated_at) = 'integer'),
	author_page_key INTEGER PRIMARY KEY
);

WITH RECURSIVE legacy_source AS (
	SELECT
		rowid AS legacy_rowid,
		typeof(id) AS id_type,
		CAST(id AS TEXT) AS text_id,
		id AS raw_id,
		user_id,
		title,
		body,
		category_slug,
		place_slug,
		created_at,
		updated_at,
		row_number() OVER (ORDER BY rowid) AS author_page_key
	FROM notes_legacy
),
legacy_count AS (
	SELECT COUNT(*) AS total FROM legacy_source
),
non_text_id_candidates(legacy_rowid, candidate, attempt) AS (
	SELECT
		legacy_rowid,
		'legacy-' || id_type || '-id-' || legacy_rowid || '-' || lower(hex(raw_id)),
		0
	FROM legacy_source
	WHERE id_type <> 'text'
	UNION ALL
	SELECT
		source.legacy_rowid,
		'legacy-' || source.id_type || '-id-' || source.legacy_rowid || '-' || lower(hex(source.raw_id)) || '-' || (candidate.attempt + 1),
		candidate.attempt + 1
	FROM non_text_id_candidates candidate
	JOIN legacy_source source ON source.legacy_rowid = candidate.legacy_rowid
	JOIN legacy_count
	WHERE candidate.attempt < legacy_count.total
		AND EXISTS (
			SELECT 1
			FROM legacy_source existing
			WHERE existing.id_type = 'text' AND existing.text_id = candidate.candidate
		)
),
resolved_non_text_ids AS (
	SELECT legacy_rowid, candidate AS id
	FROM (
		SELECT
			candidate.legacy_rowid,
			candidate.candidate,
			row_number() OVER (PARTITION BY candidate.legacy_rowid ORDER BY candidate.attempt) AS rank
		FROM non_text_id_candidates candidate
		WHERE NOT EXISTS (
			SELECT 1
			FROM legacy_source existing
			WHERE existing.id_type = 'text' AND existing.text_id = candidate.candidate
		)
	)
	WHERE rank = 1
),
legacy_notes AS (
	SELECT
		CASE
			WHEN source.id_type = 'text' THEN source.text_id
			ELSE resolved.id
		END AS id,
		source.user_id,
		source.title,
		source.body,
		source.category_slug,
		source.place_slug,
		source.created_at,
		source.updated_at,
		source.author_page_key
	FROM legacy_source source
	LEFT JOIN resolved_non_text_ids resolved ON resolved.legacy_rowid = source.legacy_rowid
)
INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at, author_page_key)
SELECT
	id,
	user_id,
	title,
	body,
	category_slug,
	place_slug,
	created_at,
	updated_at,
	author_page_key
FROM legacy_notes;

CREATE INDEX notes_recent_idx ON notes (created_at DESC, id DESC);
CREATE INDEX notes_category_idx ON notes (category_slug);
CREATE INDEX notes_place_idx ON notes (place_slug);
CREATE INDEX notes_user_idx ON notes (user_id);
CREATE INDEX notes_author_page_idx ON notes (user_id, created_at DESC, author_page_key DESC);

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

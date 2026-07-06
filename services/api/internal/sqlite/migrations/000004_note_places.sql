DROP TABLE note_search;
DROP INDEX notes_recent_idx;
DROP INDEX notes_category_idx;
DROP INDEX notes_city_idx;

ALTER TABLE notes RENAME TO notes_legacy;
ALTER TABLE categories RENAME TO categories_legacy;
ALTER TABLE places RENAME TO places_legacy;

CREATE TABLE categories (
	slug TEXT PRIMARY KEY,
	label TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
	display_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE places (
	slug TEXT PRIMARY KEY,
	label TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
	display_order INTEGER NOT NULL DEFAULT 0
);

INSERT INTO categories (slug, label, active, display_order) VALUES
	('beauty', 'Beleza', 1, 10),
	('food', 'Comida', 1, 20),
	('travel', 'Viagem', 1, 30),
	('finds', 'Achadinhos', 1, 40);

INSERT INTO categories (slug, label, active, display_order)
SELECT slug, label, active, display_order
FROM categories_legacy
WHERE slug NOT IN ('beleza', 'comida', 'viagem', 'achadinhos', 'beauty', 'food', 'travel', 'finds');

INSERT INTO places (slug, label, active, display_order) VALUES
	('sao-paulo', 'São Paulo', 1, 10),
	('rio-de-janeiro', 'Rio de Janeiro', 1, 20),
	('lisboa', 'Lisboa', 1, 30);

INSERT INTO places (slug, label, active, display_order)
SELECT slug, label, active, display_order
FROM places_legacy
WHERE slug NOT IN ('sao-paulo', 'rio-de-janeiro', 'lisboa');

INSERT INTO places (slug, label, active, display_order)
SELECT cities.slug, cities.label, 1, 0
FROM cities
WHERE cities.slug NOT IN (SELECT slug FROM places);

CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 3 AND 120),
	body TEXT NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 4000),
	category_slug TEXT NOT NULL REFERENCES categories(slug),
	place_slug TEXT REFERENCES places(slug),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

INSERT INTO notes (id, title, body, category_slug, place_slug, created_at, updated_at)
SELECT
	id,
	title,
	body,
	CASE category_slug
		WHEN 'beleza' THEN 'beauty'
		WHEN 'comida' THEN 'food'
		WHEN 'viagem' THEN 'travel'
		WHEN 'achadinhos' THEN 'finds'
		ELSE category_slug
	END,
	city_slug,
	created_at,
	updated_at
FROM notes_legacy;

CREATE INDEX notes_recent_idx ON notes (created_at DESC, id DESC);
CREATE INDEX notes_category_idx ON notes (category_slug);
CREATE INDEX notes_place_idx ON notes (place_slug);

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
DROP TABLE categories_legacy;
DROP TABLE places_legacy;
DROP TABLE cities;

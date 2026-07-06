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

INSERT OR IGNORE INTO categories (slug, label, active, display_order)
SELECT
	CASE slug
		WHEN 'beleza' THEN 'beauty'
		WHEN 'comida' THEN 'food'
		WHEN 'viagem' THEN 'travel'
		WHEN 'achadinhos' THEN 'finds'
		ELSE slug
	END,
	label,
	active,
	display_order
FROM categories_legacy
ORDER BY CASE slug
	WHEN 'beleza' THEN 0
	WHEN 'comida' THEN 0
	WHEN 'viagem' THEN 0
	WHEN 'achadinhos' THEN 0
	ELSE 1
END;

INSERT INTO categories (slug, label, active, display_order)
SELECT 'beauty', 'Beleza', 1, 10
WHERE NOT EXISTS (SELECT 1 FROM categories WHERE slug = 'beauty');

INSERT INTO categories (slug, label, active, display_order)
SELECT 'food', 'Comida', 1, 20
WHERE NOT EXISTS (SELECT 1 FROM categories WHERE slug = 'food');

INSERT INTO categories (slug, label, active, display_order)
SELECT 'travel', 'Viagem', 1, 30
WHERE NOT EXISTS (SELECT 1 FROM categories WHERE slug = 'travel');

INSERT INTO categories (slug, label, active, display_order)
SELECT 'finds', 'Achadinhos', 1, 40
WHERE NOT EXISTS (SELECT 1 FROM categories WHERE slug = 'finds');

INSERT INTO places (slug, label, active, display_order)
SELECT slug, label, active, display_order
FROM places_legacy
ORDER BY display_order ASC, label ASC, slug ASC;

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

CREATE TABLE categories (
	slug TEXT PRIMARY KEY,
	label TEXT NOT NULL
);

CREATE TABLE cities (
	slug TEXT PRIMARY KEY,
	label TEXT NOT NULL
);

INSERT INTO categories (slug, label) VALUES
	('beleza', 'Beleza'),
	('comida', 'Comida'),
	('viagem', 'Viagem'),
	('achadinhos', 'Achadinhos');

INSERT INTO cities (slug, label) VALUES
	('sao-paulo', 'São Paulo'),
	('rio-de-janeiro', 'Rio de Janeiro'),
	('lisboa', 'Lisboa');

CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 3 AND 120),
	body TEXT NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 4000),
	category_slug TEXT NOT NULL REFERENCES categories(slug),
	city_slug TEXT NOT NULL REFERENCES cities(slug),
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE INDEX notes_recent_idx ON notes (created_at DESC, id DESC);
CREATE INDEX notes_category_idx ON notes (category_slug);
CREATE INDEX notes_city_idx ON notes (city_slug);

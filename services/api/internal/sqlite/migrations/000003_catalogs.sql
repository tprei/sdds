ALTER TABLE categories ADD COLUMN active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1));
ALTER TABLE categories ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;

UPDATE categories
SET display_order = CASE slug
	WHEN 'beleza' THEN 10
	WHEN 'comida' THEN 20
	WHEN 'viagem' THEN 30
	WHEN 'achadinhos' THEN 40
	ELSE 0
END;

CREATE TABLE places (
	slug TEXT PRIMARY KEY,
	label TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
	display_order INTEGER NOT NULL DEFAULT 0
);

INSERT INTO places (slug, label, active, display_order) VALUES
	('sao-paulo', 'São Paulo', 1, 10),
	('rio-de-janeiro', 'Rio de Janeiro', 1, 20),
	('lisboa', 'Lisboa', 1, 30);

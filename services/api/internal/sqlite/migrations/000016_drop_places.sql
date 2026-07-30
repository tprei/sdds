-- The place domain (three seeded rows -- sao-paulo, rio-de-janeiro, and a
-- non-Brazilian lisboa -- behind a places catalog) is dead product surface:
-- location UI was removed by owner decision and #138 closed not-planned.
-- The `cities` table this once referenced was already folded into `places`
-- and dropped by 000004_note_places; only `places` and notes.place_slug
-- remain to remove here.
--
-- notes.place_slug carries both a covering index and a foreign key into
-- places(slug). Earlier migrations that touch the notes table (000004,
-- 000006, 000008) recreate it with a "RENAME TO notes_legacy" dance, but
-- that pattern is unsafe now: note_images, image_uploads,
-- note_create_requests, note_useful_reactions, and note_comments all hold
-- their own "REFERENCES notes(id)" foreign keys, and SQLite rewrites those
-- definitions to point at the renamed-away table -- without ever rewriting
-- them back -- which would permanently break every one of them once
-- notes_legacy is dropped. ALTER TABLE ... DROP COLUMN (supported since
-- SQLite 3.35, and by this repo's modernc.org/sqlite driver) removes the
-- column and its foreign key in place without touching the notes table's
-- identity, so none of its children are put at risk.
DROP INDEX notes_place_idx;

ALTER TABLE notes DROP COLUMN place_slug;

DROP TABLE places;

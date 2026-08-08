-- Deleting a note detaches its consumed uploads by consumed_note_id. Without
-- this index that UPDATE scans every upload row on every note delete. The
-- partial predicate keeps the index to the small consumed set.
CREATE INDEX image_uploads_note_reclaim_idx
	ON image_uploads (consumed_note_id)
	WHERE consumed_note_id IS NOT NULL;

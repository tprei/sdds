CREATE INDEX notes_author_page_idx ON notes (user_id, created_at DESC, id DESC);

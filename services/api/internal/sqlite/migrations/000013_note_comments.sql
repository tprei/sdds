CREATE TABLE note_comments (
    id               TEXT NOT NULL UNIQUE CHECK (typeof(id) = 'text'),
    note_id          TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body             TEXT NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 1000),
    created_at       INTEGER NOT NULL CHECK (typeof(created_at) = 'integer'),
    comment_page_key INTEGER PRIMARY KEY AUTOINCREMENT
);

CREATE INDEX note_comments_note_page_idx
    ON note_comments (note_id, comment_page_key);

CREATE INDEX note_comments_user_idx
    ON note_comments (user_id);

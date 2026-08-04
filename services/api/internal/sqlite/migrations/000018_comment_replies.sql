-- A reply points at exactly one top-level comment. SQLite cannot express the
-- cross-row "parent must itself be top-level" rule, so the comment store
-- enforces it inside the create-reply transaction. The column-level guards
-- here cover the rest: the parent must exist, a comment cannot be its own
-- parent, and deleting a parent deletes its replies.
ALTER TABLE note_comments
    ADD COLUMN parent_comment_id TEXT
        REFERENCES note_comments(id) ON DELETE CASCADE
        CHECK (parent_comment_id IS NULL OR parent_comment_id <> id);

CREATE INDEX note_comments_parent_page_idx
    ON note_comments (parent_comment_id, comment_page_key);

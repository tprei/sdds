CREATE TABLE note_embeddings (
    note_id        TEXT PRIMARY KEY REFERENCES notes(id) ON DELETE CASCADE,
    model_id       TEXT NOT NULL,
    model_revision TEXT NOT NULL,
    dimension      INTEGER NOT NULL CHECK (dimension > 0),
    source_sha256  TEXT NOT NULL CHECK (length(source_sha256) = 64),
    vector         BLOB NOT NULL CHECK (length(vector) = dimension * 4),
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE INDEX note_embeddings_model_idx ON note_embeddings (model_id, model_revision);

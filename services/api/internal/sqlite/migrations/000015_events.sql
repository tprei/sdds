-- The allowlist mirrors event.Kind. Adding a kind requires a versioned migration
-- so old readers never silently accept an event they cannot interpret.

-- Domain validation owns the closed vocabularies; SQLite keeps these values as
-- bounded TEXT so event storage remains append-only and queryable.
CREATE TABLE events (
    event_page_key INTEGER PRIMARY KEY AUTOINCREMENT,
    id            TEXT NOT NULL UNIQUE CHECK (typeof(id) = 'text' AND length(trim(id)) > 0),
    kind          TEXT NOT NULL CHECK (kind IN (
        'explore_notes_impression',
        'explore_note_opened',
        'search_submitted',
        'search_results_impression',
        'search_result_opened',
        'search_reformulated',
        'search_no_results',
        'note_marked_useful',
        'note_unmarked_useful',
        'comment_created',
        'report_created',
        'note_published'
    )),
    occurred_at   INTEGER NOT NULL CHECK (typeof(occurred_at) = 'integer'),
    received_at   INTEGER NOT NULL CHECK (typeof(received_at) = 'integer'),
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id TEXT CHECK (installation_id IS NULL OR (typeof(installation_id) = 'text' AND length(trim(installation_id)) > 0)),
    app_platform  TEXT NOT NULL CHECK (app_platform IN ('ios', 'android', 'web')),
    app_version   TEXT CHECK (app_version IS NULL OR (typeof(app_version) = 'text' AND length(app_version) BETWEEN 1 AND 64)),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    search_id     TEXT CHECK (search_id IS NULL OR (typeof(search_id) = 'text' AND length(trim(search_id)) > 0)),
    payload_json  TEXT NOT NULL CHECK (typeof(payload_json) = 'text' AND json_valid(payload_json) = 1 AND json_type(payload_json) = 'object')
);

CREATE INDEX events_received_page_idx ON events (received_at, event_page_key);
CREATE INDEX events_occurred_page_idx ON events (occurred_at, event_page_key);
CREATE INDEX events_kind_occurred_page_idx ON events (kind, occurred_at, event_page_key);
CREATE INDEX events_search_occurred_page_idx ON events (search_id, occurred_at, event_page_key) WHERE search_id IS NOT NULL;
CREATE INDEX events_user_occurred_page_idx ON events (user_id, occurred_at, event_page_key) WHERE user_id IS NOT NULL;
CREATE INDEX events_installation_occurred_page_idx ON events (installation_id, occurred_at, event_page_key) WHERE installation_id IS NOT NULL;

CREATE TABLE reports (
    id               TEXT NOT NULL UNIQUE CHECK (typeof(id) = 'text'),
    reporter_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type      TEXT NOT NULL CHECK (target_type IN ('note', 'comment')),
    target_id        TEXT NOT NULL CHECK (typeof(target_id) = 'text' AND length(trim(target_id)) > 0),
    reason           TEXT NOT NULL CHECK (reason IN (
        'spam',
        'harassment',
        'harmful_or_misleading',
        'other'
    )),
    details          TEXT CHECK (
        details IS NULL OR length(trim(details)) BETWEEN 1 AND 1000
    ),
    created_at       INTEGER NOT NULL CHECK (typeof(created_at) = 'integer'),
    report_page_key  INTEGER PRIMARY KEY AUTOINCREMENT,
    UNIQUE (reporter_user_id, target_type, target_id)
);

CREATE INDEX reports_created_idx ON reports (report_page_key);
CREATE INDEX reports_target_idx ON reports (target_type, target_id);

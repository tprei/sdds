CREATE TABLE orphaned_media_objects (
	storage_key TEXT NOT NULL PRIMARY KEY CHECK (typeof(storage_key) = 'text' AND length(storage_key) > 0),
	orphaned_at INTEGER NOT NULL CHECK (typeof(orphaned_at) = 'integer')
);
CREATE INDEX orphaned_media_objects_sweep_idx ON orphaned_media_objects (orphaned_at, storage_key);

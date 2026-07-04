package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const (
	enableForeignKeysSQL = "PRAGMA foreign_keys = ON"
	setBusyTimeoutSQL    = "PRAGMA busy_timeout = 5000"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(enableForeignKeysSQL); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("enable foreign keys: %w; close sqlite: %v", err, closeErr)
		}
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(setBusyTimeoutSQL); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("set busy timeout: %w; close sqlite: %v", err, closeErr)
		}
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	return db, nil
}

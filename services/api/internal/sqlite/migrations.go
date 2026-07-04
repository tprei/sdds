package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	createSchemaMigrationsSQL = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`
	recordMigrationSQL = `INSERT INTO schema_migrations (version) VALUES (?)`
	checkMigrationSQL  = `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`
)

func ApplyMigrations(ctx context.Context, db *sql.DB) (err error) {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback migrations: %w", rollbackErr)
		}
	}()

	if _, err := tx.ExecContext(ctx, createSchemaMigrationsSQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
		applied, err := migrationApplied(ctx, tx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		contents, err := migrations.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, recordMigrationSQL, version); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}

func migrationApplied(ctx context.Context, tx *sql.Tx, version string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, checkMigrationSQL, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return count > 0, nil
}

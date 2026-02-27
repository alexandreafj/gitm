package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS repositories (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    name           TEXT     NOT NULL,
    alias          TEXT     NOT NULL UNIQUE,
    path           TEXT     NOT NULL UNIQUE,
    default_branch TEXT     NOT NULL DEFAULT 'main',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// migrations are applied once, in order. Each entry is a SQL statement.
var migrations = []string{
	// v1: add alias column (display name / user-facing identifier).
	// Backfill existing rows with their current name so nothing breaks.
	`ALTER TABLE repositories ADD COLUMN alias TEXT NOT NULL DEFAULT ''`,
	`UPDATE repositories SET alias = name WHERE alias = ''`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_repositories_alias ON repositories (alias)`,
	// v2: drop the UNIQUE constraint on the name column so two repos with the
	// same directory name (e.g. v1 under different parents) can be registered
	// with different aliases. SQLite requires recreating the table to drop a
	// constraint — we use the standard "rename-old / create-new / copy / drop" pattern.
	`CREATE TABLE IF NOT EXISTS repositories_new (
		id             INTEGER  PRIMARY KEY AUTOINCREMENT,
		name           TEXT     NOT NULL,
		alias          TEXT     NOT NULL UNIQUE,
		path           TEXT     NOT NULL UNIQUE,
		default_branch TEXT     NOT NULL DEFAULT 'main',
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`INSERT OR IGNORE INTO repositories_new (id, name, alias, path, default_branch, created_at)
		SELECT id, name, alias, path, default_branch, created_at FROM repositories`,
	`DROP TABLE IF EXISTS repositories_old`,
	`ALTER TABLE repositories RENAME TO repositories_old`,
	`ALTER TABLE repositories_new RENAME TO repositories`,
	`DROP TABLE IF EXISTS repositories_old`,
}

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer to avoid SQLITE_BUSY on concurrent writes.
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs the schema creation and all incremental migrations.
// Migrations are idempotent: ALTER TABLE on an existing column returns an
// "duplicate column" error which we silently ignore.
func (db *DB) migrate() error {
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, stmt := range migrations {
		if _, err := db.conn.Exec(stmt); err != nil {
			// SQLite returns "duplicate column name" when ADD COLUMN is re-run;
			// ignore that specific error so migrations stay idempotent.
			if !isDuplicateColumn(err) && !isIndexAlreadyExists(err) {
				return fmt.Errorf("migrate: %w", err)
			}
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

func isIndexAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}

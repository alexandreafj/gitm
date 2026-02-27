package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS repositories (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    name           TEXT     NOT NULL UNIQUE,
    path           TEXT     NOT NULL UNIQUE,
    default_branch TEXT     NOT NULL DEFAULT 'main',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

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

// migrate runs the schema migrations.
func (db *DB) migrate() error {
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

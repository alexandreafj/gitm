package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// schema is the base table created before incremental migrations run. The
// groups and group_repositories tables are intentionally NOT created here: they
// must be created by the v3 migration, which runs AFTER the v2 rebuild of the
// repositories table. Creating group_repositories up front lets v2's
// `ALTER TABLE repositories RENAME TO repositories_old` rewrite its foreign key
// to point at the soon-to-be-dropped repositories_old table.
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
	// v3: add many-to-many repository groups. The built-in "all" group is
	// materialized so upgraded users can see every existing repo in it without
	// any manual migration step.
	`CREATE TABLE IF NOT EXISTS groups (
		id         INTEGER  PRIMARY KEY AUTOINCREMENT,
		name       TEXT     NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS group_repositories (
		group_id      INTEGER NOT NULL,
		repository_id INTEGER NOT NULL,
		PRIMARY KEY (group_id, repository_id),
		FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
		FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
	)`,
	`INSERT OR IGNORE INTO groups (name) VALUES ('all')`,
	`INSERT OR IGNORE INTO group_repositories (group_id, repository_id)
		SELECT groups.id, repositories.id
		FROM groups
		CROSS JOIN repositories
		WHERE groups.name = 'all'`,
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

// migrate runs the base schema then any not-yet-applied incremental migrations.
// PRAGMA user_version records how many migration statements have been applied so
// each runs exactly once. This matters because some migrations (e.g. the v2
// table rebuild) are destructive and must not re-run on every startup. Existing
// databases predate version tracking and start at user_version 0, so the first
// run with this code re-applies all statements — the statements are idempotent
// (duplicate-column / already-exists errors are ignored), so that is safe.
func (db *DB) migrate() error {
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	var version int
	if err := db.conn.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if _, err := db.conn.Exec(migrations[i]); err != nil {
			// SQLite returns "duplicate column name" when ADD COLUMN is re-run;
			// ignore that specific error so migrations stay idempotent.
			if !isDuplicateColumn(err) && !isIndexAlreadyExists(err) {
				return fmt.Errorf("migrate: %w", err)
			}
		}
	}

	// PRAGMA user_version does not accept bound parameters; len(migrations) is a
	// trusted integer, so formatting it directly is safe.
	if _, err := db.conn.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

func isIndexAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}

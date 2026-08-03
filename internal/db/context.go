package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DefaultContextName is the built-in context that repositories belong to
// unless they are moved to a custom context.
const DefaultContextName = "default"

// activeContextKey is the settings key storing the active context id.
const activeContextKey = "active_context_id"

var (
	// ErrReservedContext is returned when callers try to mutate the built-in context.
	ErrReservedContext = errors.New("reserved context")
	// ErrInvalidContextName is returned when a context name is empty or unsupported.
	ErrInvalidContextName = errors.New("invalid context name")
	// ErrContextNotFound is returned when a context lookup finds no matching context.
	ErrContextNotFound = errors.New("context not found")
	// ErrContextActive is returned when deleting the currently active context.
	ErrContextActive = errors.New("context is active")
)

// Context represents a repository context. Each repository belongs to exactly
// one context, and multi-repo commands operate only on the active context.
type Context struct {
	ID        int64
	Name      string
	RepoCount int
	Active    bool
	CreatedAt time.Time
}

// CreateContext creates a custom repository context.
func (db *DB) CreateContext(name string) (*Context, error) {
	contextName, err := validateCustomContextName(name)
	if err != nil {
		return nil, err
	}
	res, err := db.conn.Exec(`INSERT INTO contexts (name) VALUES (?)`, contextName)
	if err != nil {
		return nil, fmt.Errorf("create context %q: %w", contextName, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read context id: %w", err)
	}
	return &Context{ID: id, Name: contextName}, nil
}

// GetContext returns a context by name.
func (db *DB) GetContext(name string) (*Context, error) {
	contextName, err := normalizeGroupName(name)
	if err != nil {
		return nil, ErrInvalidContextName
	}
	activeID, err := activeContextID(db.conn)
	if err != nil {
		return nil, err
	}
	row := db.conn.QueryRow(contextSelectSQL()+` WHERE c.name = ? GROUP BY c.id, c.name, c.created_at`, contextName)
	return scanContext(row, activeID)
}

// ListContexts returns all contexts ordered with the built-in default context first.
func (db *DB) ListContexts() ([]*Context, error) {
	activeID, err := activeContextID(db.conn)
	if err != nil {
		return nil, err
	}
	rows, err := db.conn.Query(contextSelectSQL() +
		` GROUP BY c.id, c.name, c.created_at
		  ORDER BY CASE WHEN c.name = 'default' THEN 0 ELSE 1 END, c.name`)
	if err != nil {
		return nil, fmt.Errorf("list contexts: %w", err)
	}
	defer rows.Close()

	var contexts []*Context
	for rows.Next() {
		context, err := scanContext(rows, activeID)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, context)
	}
	return contexts, rows.Err()
}

// ActiveContext returns the currently active context. When no context has been
// explicitly activated (or the stored one no longer exists), the built-in
// default context is returned.
func (db *DB) ActiveContext() (*Context, error) {
	activeID, err := activeContextID(db.conn)
	if err != nil {
		return nil, err
	}
	row := db.conn.QueryRow(contextSelectSQL()+` WHERE c.id = ? GROUP BY c.id, c.name, c.created_at`, activeID)
	context, err := scanContext(row, activeID)
	if errors.Is(err, ErrContextNotFound) {
		// Stored id points at a deleted context; fall back to default.
		row = db.conn.QueryRow(contextSelectSQL()+` WHERE c.name = ? GROUP BY c.id, c.name, c.created_at`, DefaultContextName)
		return scanContext(row, -1)
	}
	return context, err
}

// SetActiveContext switches the active context by name.
func (db *DB) SetActiveContext(name string) (*Context, error) {
	context, err := db.GetContext(name)
	if err != nil {
		return nil, err
	}
	if _, err := db.conn.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		activeContextKey, fmt.Sprintf("%d", context.ID),
	); err != nil {
		return nil, fmt.Errorf("set active context %q: %w", context.Name, err)
	}
	context.Active = true
	return context, nil
}

// RenameContext renames a custom context. The built-in default context cannot
// be renamed.
func (db *DB) RenameContext(oldName, newName string) error {
	oldContextName, err := validateCustomContextName(oldName)
	if err != nil {
		return err
	}
	newContextName, err := validateCustomContextName(newName)
	if err != nil {
		return err
	}
	res, err := db.conn.Exec(`UPDATE contexts SET name = ? WHERE name = ?`, newContextName, oldContextName)
	if err != nil {
		return fmt.Errorf("rename context %q to %q: %w", oldContextName, newContextName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count renamed contexts: %w", err)
	}
	if n == 0 {
		return ErrContextNotFound
	}
	return nil
}

// DeleteContext deletes a custom context. Its repositories are moved back to
// the built-in default context, never deleted. The active context and the
// default context cannot be deleted.
func (db *DB) DeleteContext(name string) (moved int64, err error) {
	contextName, err := validateCustomContextName(name)
	if err != nil {
		return 0, err
	}
	active, err := db.ActiveContext()
	if err != nil {
		return 0, err
	}
	if active.Name == contextName {
		return 0, ErrContextActive
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete context: %w", err)
	}
	//nolint:errcheck // rollback is best-effort; after commit it is expected to fail.
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE repositories
		 SET context_id = (SELECT id FROM contexts WHERE name = ?)
		 WHERE context_id IN (SELECT id FROM contexts WHERE name = ?)`,
		DefaultContextName, contextName,
	)
	if err != nil {
		return 0, fmt.Errorf("move repositories to %q context: %w", DefaultContextName, err)
	}
	moved, err = res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count moved repositories: %w", err)
	}

	res, err = tx.Exec(`DELETE FROM contexts WHERE name = ?`, contextName)
	if err != nil {
		return 0, fmt.Errorf("delete context %q: %w", contextName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted contexts: %w", err)
	}
	if n == 0 {
		return 0, ErrContextNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete context: %w", err)
	}
	return moved, nil
}

// AssignRepositoriesToContext moves repositories into a context by alias.
// A repository belongs to exactly one context, so this removes each repo from
// its previous context implicitly.
func (db *DB) AssignRepositoriesToContext(contextName string, aliases []string) error {
	name, err := normalizeGroupName(contextName)
	if err != nil {
		return ErrInvalidContextName
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin assign repositories to context: %w", err)
	}
	//nolint:errcheck // rollback is best-effort; after commit it is expected to fail.
	defer tx.Rollback()

	contextID, err := contextIDByName(tx, name)
	if err != nil {
		return err
	}
	repoIDs, err := repositoryIDsByAlias(tx, aliases)
	if err != nil {
		return err
	}
	for _, repoID := range repoIDs {
		if _, err := tx.Exec(
			`UPDATE repositories SET context_id = ? WHERE id = ?`,
			contextID, repoID,
		); err != nil {
			return fmt.Errorf("assign repository to context %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit assign repositories to context: %w", err)
	}
	return nil
}

// ListRepositoriesByContext returns repositories in a context ordered by alias.
func (db *DB) ListRepositoriesByContext(contextName string) ([]*Repository, error) {
	name, err := normalizeGroupName(contextName)
	if err != nil {
		return nil, ErrInvalidContextName
	}
	contextID, err := contextIDByName(db.conn, name)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(
		`SELECT id, name, alias, path, default_branch, COALESCE(context_id, 0), created_at
		 FROM repositories
		 WHERE context_id = ?
		 ORDER BY alias`,
		contextID,
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories by context: %w", err)
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		repo, err := scanRepository(rows)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

// activeContextID returns the stored active context id, or the default
// context's id when none has been stored yet.
func activeContextID(q queryer) (int64, error) {
	var value int64
	err := q.QueryRow(`SELECT value FROM settings WHERE key = ?`, activeContextKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return contextIDByName(q, DefaultContextName)
	}
	if err != nil {
		return 0, fmt.Errorf("read active context: %w", err)
	}
	return value, nil
}

func contextSelectSQL() string {
	return `SELECT c.id, c.name, COUNT(r.id), c.created_at
		FROM contexts c
		LEFT JOIN repositories r ON r.context_id = c.id`
}

func scanContext(s scanner, activeID int64) (*Context, error) {
	var context Context
	var createdAt string
	err := s.Scan(&context.ID, &context.Name, &context.RepoCount, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrContextNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan context: %w", err)
	}
	//nolint:errcheck // time.Parse failure leaves CreatedAt as zero value, which is safe
	context.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	context.Active = context.ID == activeID
	return &context, nil
}

func validateCustomContextName(name string) (string, error) {
	contextName, err := normalizeGroupName(name)
	if err != nil {
		return "", ErrInvalidContextName
	}
	if contextName == DefaultContextName {
		return "", ErrReservedContext
	}
	return contextName, nil
}

func contextIDByName(q queryer, name string) (int64, error) {
	var id int64
	err := q.QueryRow(`SELECT id FROM contexts WHERE name = ?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrContextNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lookup context %q: %w", name, err)
	}
	return id, nil
}

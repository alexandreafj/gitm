package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a repository is not found.
var ErrNotFound = errors.New("repository not found")

// Repository represents a registered git repository.
type Repository struct {
	ID            int64
	Name          string // auto-detected base directory name
	Alias         string // user-facing display name (unique); defaults to Name
	Path          string
	DefaultBranch string
	CreatedAt     time.Time
}

// AddRepository inserts a new repository record.
// alias is the display name shown in all output; pass "" to default to name.
func (db *DB) AddRepository(name, alias, path, defaultBranch string) (*Repository, error) {
	if alias == "" {
		alias = name
	}
	res, err := db.conn.Exec(
		`INSERT INTO repositories (name, alias, path, default_branch) VALUES (?, ?, ?, ?)`,
		name, alias, path, defaultBranch,
	)
	if err != nil {
		return nil, fmt.Errorf("add repository: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Repository{
		ID:            id,
		Name:          name,
		Alias:         alias,
		Path:          path,
		DefaultBranch: defaultBranch,
	}, nil
}

// GetRepository returns a single repository by alias.
func (db *DB) GetRepository(alias string) (*Repository, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, alias, path, default_branch, created_at FROM repositories WHERE alias = ?`,
		alias,
	)
	return scanRepository(row)
}

// GetRepositoryByPath returns a single repository by its filesystem path.
func (db *DB) GetRepositoryByPath(path string) (*Repository, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, alias, path, default_branch, created_at FROM repositories WHERE path = ?`,
		path,
	)
	return scanRepository(row)
}

// ListRepositories returns all registered repositories ordered by alias.
func (db *DB) ListRepositories() ([]*Repository, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, alias, path, default_branch, created_at FROM repositories ORDER BY alias`,
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
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

// RemoveRepository deletes a repository by alias.
func (db *DB) RemoveRepository(alias string) error {
	res, err := db.conn.Exec(`DELETE FROM repositories WHERE alias = ?`, alias)
	if err != nil {
		return fmt.Errorf("remove repository: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RenameRepository changes the alias of a registered repository.
func (db *DB) RenameRepository(oldAlias, newAlias string) error {
	res, err := db.conn.Exec(
		`UPDATE repositories SET alias = ? WHERE alias = ?`,
		newAlias, oldAlias,
	)
	if err != nil {
		return fmt.Errorf("rename repository: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateDefaultBranch updates the stored default branch for a repository.
func (db *DB) UpdateDefaultBranch(alias, branch string) error {
	_, err := db.conn.Exec(
		`UPDATE repositories SET default_branch = ? WHERE alias = ?`,
		branch, alias,
	)
	return err
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRepository(s scanner) (*Repository, error) {
	var r Repository
	var createdAt string
	err := s.Scan(&r.ID, &r.Name, &r.Alias, &r.Path, &r.DefaultBranch, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan repository: %w", err)
	}
	r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &r, nil
}

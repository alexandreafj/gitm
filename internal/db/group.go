package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// DefaultGroupName is the built-in group containing all registered repos.
const DefaultGroupName = "all"

var (
	// ErrReservedGroup is returned when callers try to mutate the built-in group.
	ErrReservedGroup = errors.New("reserved group")
	// ErrInvalidGroupName is returned when a group name is empty or unsupported.
	ErrInvalidGroupName = errors.New("invalid group name")
)

// Group represents a repository group.
type Group struct {
	ID        int64
	Name      string
	RepoCount int
	CreatedAt time.Time
}

// CreateGroup creates a custom repository group.
func (db *DB) CreateGroup(name string) (*Group, error) {
	groupName, err := validateCustomGroupName(name)
	if err != nil {
		return nil, err
	}
	res, err := db.conn.Exec(`INSERT INTO groups (name) VALUES (?)`, groupName)
	if err != nil {
		return nil, fmt.Errorf("create group %q: %w", groupName, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read group id: %w", err)
	}
	return &Group{ID: id, Name: groupName}, nil
}

// GetGroup returns a group by name.
func (db *DB) GetGroup(name string) (*Group, error) {
	groupName, err := normalizeGroupName(name)
	if err != nil {
		return nil, err
	}
	row := db.conn.QueryRow(groupSelectBaseSQL()+` WHERE g.name = ?`+groupSelectGroupBySQL(), groupName)
	return scanGroup(row)
}

// ListGroups returns all groups ordered with the built-in all group first.
func (db *DB) ListGroups() ([]*Group, error) {
	rows, err := db.conn.Query(groupSelectBaseSQL() + groupSelectGroupBySQL() + ` ORDER BY CASE WHEN g.name = 'all' THEN 0 ELSE 1 END, g.name`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// RenameGroup renames a custom group.
func (db *DB) RenameGroup(oldName, newName string) error {
	oldGroupName, err := validateCustomGroupName(oldName)
	if err != nil {
		return err
	}
	newGroupName, err := validateCustomGroupName(newName)
	if err != nil {
		return err
	}
	res, err := db.conn.Exec(`UPDATE groups SET name = ? WHERE name = ?`, newGroupName, oldGroupName)
	if err != nil {
		return fmt.Errorf("rename group %q to %q: %w", oldGroupName, newGroupName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count renamed groups: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteGroup deletes a custom group and its memberships.
func (db *DB) DeleteGroup(name string) error {
	groupName, err := validateCustomGroupName(name)
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin delete group: %w", err)
	}
	//nolint:errcheck // rollback is best-effort; after commit it is expected to fail.
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM group_repositories
		 WHERE group_id IN (SELECT id FROM groups WHERE name = ?)`,
		groupName,
	); err != nil {
		return fmt.Errorf("delete group memberships: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM groups WHERE name = ?`, groupName)
	if err != nil {
		return fmt.Errorf("delete group %q: %w", groupName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted groups: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete group: %w", err)
	}
	return nil
}

// AddRepositoriesToGroup adds repositories to a custom group by alias.
func (db *DB) AddRepositoriesToGroup(groupName string, aliases []string) error {
	name, err := validateCustomGroupName(groupName)
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin add repositories to group: %w", err)
	}
	//nolint:errcheck // rollback is best-effort; after commit it is expected to fail.
	defer tx.Rollback()

	groupID, err := groupIDByName(tx, name)
	if err != nil {
		return err
	}
	repoIDs, err := repositoryIDsByAlias(tx, aliases)
	if err != nil {
		return err
	}
	for _, repoID := range repoIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO group_repositories (group_id, repository_id) VALUES (?, ?)`,
			groupID, repoID,
		); err != nil {
			return fmt.Errorf("add repository to group %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add repositories to group: %w", err)
	}
	return nil
}

// RemoveRepositoriesFromGroup removes repositories from a custom group by alias.
func (db *DB) RemoveRepositoriesFromGroup(groupName string, aliases []string) error {
	name, err := validateCustomGroupName(groupName)
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin remove repositories from group: %w", err)
	}
	//nolint:errcheck // rollback is best-effort; after commit it is expected to fail.
	defer tx.Rollback()

	groupID, err := groupIDByName(tx, name)
	if err != nil {
		return err
	}
	repoIDs, err := repositoryIDsByAlias(tx, aliases)
	if err != nil {
		return err
	}
	for _, repoID := range repoIDs {
		if _, err := tx.Exec(
			`DELETE FROM group_repositories WHERE group_id = ? AND repository_id = ?`,
			groupID, repoID,
		); err != nil {
			return fmt.Errorf("remove repository from group %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove repositories from group: %w", err)
	}
	return nil
}

// ListRepositoriesByGroup returns repositories in a group ordered by alias.
func (db *DB) ListRepositoriesByGroup(groupName string) ([]*Repository, error) {
	name, err := normalizeGroupName(groupName)
	if err != nil {
		return nil, err
	}
	groupID, err := db.groupID(name)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(
		`SELECT r.id, r.name, r.alias, r.path, r.default_branch, r.created_at
		 FROM repositories r
		 JOIN group_repositories gr ON gr.repository_id = r.id
		 WHERE gr.group_id = ?
		 ORDER BY r.alias`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories by group: %w", err)
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

func groupSelectBaseSQL() string {
	return `SELECT g.id, g.name, COUNT(gr.repository_id), g.created_at
		FROM groups g
		LEFT JOIN group_repositories gr ON gr.group_id = g.id`
}

func groupSelectGroupBySQL() string {
	return ` GROUP BY g.id, g.name, g.created_at`
}

func scanGroup(s scanner) (*Group, error) {
	var group Group
	var createdAt string
	err := s.Scan(&group.ID, &group.Name, &group.RepoCount, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan group: %w", err)
	}
	//nolint:errcheck // time.Parse failure leaves CreatedAt as zero value, which is safe
	group.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &group, nil
}

func normalizeGroupName(name string) (string, error) {
	groupName := strings.TrimSpace(name)
	if groupName == "" {
		return "", ErrInvalidGroupName
	}
	for _, r := range groupName {
		if unicode.IsSpace(r) || r == ',' {
			return "", ErrInvalidGroupName
		}
	}
	return groupName, nil
}

func validateCustomGroupName(name string) (string, error) {
	groupName, err := normalizeGroupName(name)
	if err != nil {
		return "", err
	}
	if groupName == DefaultGroupName {
		return "", ErrReservedGroup
	}
	return groupName, nil
}

func (db *DB) groupID(name string) (int64, error) {
	return groupIDByName(db.conn, name)
}

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func groupIDByName(q queryer, name string) (int64, error) {
	var id int64
	err := q.QueryRow(`SELECT id FROM groups WHERE name = ?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lookup group %q: %w", name, err)
	}
	return id, nil
}

func repositoryIDsByAlias(tx *sql.Tx, aliases []string) ([]int64, error) {
	seen := make(map[string]bool, len(aliases))
	ids := make([]int64, 0, len(aliases))
	for _, alias := range aliases {
		if seen[alias] {
			continue
		}
		seen[alias] = true
		var id int64
		err := tx.QueryRow(`SELECT id FROM repositories WHERE alias = ?`, alias).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lookup repository %q: %w", alias, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

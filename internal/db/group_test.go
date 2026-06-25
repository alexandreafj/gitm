package db_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func createLegacyRepositoriesDB(t *testing.T, dbPath string) {
	t.Helper()
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer conn.Close()

	_, err = conn.Exec(`
CREATE TABLE repositories (
	id             INTEGER  PRIMARY KEY AUTOINCREMENT,
	name           TEXT     NOT NULL,
	alias          TEXT     NOT NULL UNIQUE,
	path           TEXT     NOT NULL UNIQUE,
	default_branch TEXT     NOT NULL DEFAULT 'main',
	created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO repositories (name, alias, path, default_branch) VALUES
	('repo1', 'repo1', '/path/repo1', 'main'),
	('repo2', 'repo2', '/path/repo2', 'master');
`)
	if err != nil {
		t.Fatalf("create legacy db: %v", err)
	}
}

func TestOpenMigratesGroupsForExistingRepositories(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy.db")
	createLegacyRepositoriesDB(t, dbPath)

	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open legacy db: %v", err)
	}
	defer d.Close()

	groups, err := d.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != db.DefaultGroupName {
		t.Fatalf("groups = %#v, want only %q", groups, db.DefaultGroupName)
	}
	if groups[0].RepoCount != 2 {
		t.Fatalf("all group repo count = %d, want 2", groups[0].RepoCount)
	}

	repos, err := d.ListRepositoriesByGroup(db.DefaultGroupName)
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup(all): %v", err)
	}
	if got, want := aliasesOf(repos), []string{"repo1", "repo2"}; !sameStrings(got, want) {
		t.Fatalf("all group repos = %v, want %v", got, want)
	}
}

func TestOpenGroupsMigrationIsIdempotent(t *testing.T) {
	d, dbPath := initDB(t)
	if _, err := d.AddRepository("repo1", "repo1", "/path/repo1", "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer reopened.Close()

	groups, err := reopened.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != db.DefaultGroupName || groups[0].RepoCount != 1 {
		t.Fatalf("groups after reopen = %#v", groups)
	}

	repos, err := reopened.ListRepositoriesByGroup(db.DefaultGroupName)
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup(all): %v", err)
	}
	if got, want := aliasesOf(repos), []string{"repo1"}; !sameStrings(got, want) {
		t.Fatalf("all group repos after reopen = %v, want %v", got, want)
	}
}

func TestAddRepositoryAddsDefaultGroupMembership(t *testing.T) {
	d, _ := initDB(t)

	if _, err := d.AddRepository("repo1", "repo1", "/path/repo1", "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	repos, err := d.ListRepositoriesByGroup(db.DefaultGroupName)
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup(all): %v", err)
	}
	if got, want := aliasesOf(repos), []string{"repo1"}; !sameStrings(got, want) {
		t.Fatalf("all group repos = %v, want %v", got, want)
	}
}

func TestGroupCRUDAndMembership(t *testing.T) {
	d, _ := initDB(t)
	if _, err := d.AddRepository("repo1", "repo1", "/path/repo1", "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	if _, err := d.AddRepository("repo2", "repo2", "/path/repo2", "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	group, err := d.CreateGroup("backend")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if group.Name != "backend" || group.RepoCount != 0 {
		t.Fatalf("created group = %#v", group)
	}

	if err := d.AddRepositoriesToGroup("backend", []string{"repo2", "repo1", "repo1"}); err != nil {
		t.Fatalf("AddRepositoriesToGroup: %v", err)
	}
	repos, err := d.ListRepositoriesByGroup("backend")
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup: %v", err)
	}
	if got, want := aliasesOf(repos), []string{"repo1", "repo2"}; !sameStrings(got, want) {
		t.Fatalf("backend repos = %v, want %v", got, want)
	}

	if err := d.RemoveRepositoriesFromGroup("backend", []string{"repo1"}); err != nil {
		t.Fatalf("RemoveRepositoriesFromGroup: %v", err)
	}
	repos, err = d.ListRepositoriesByGroup("backend")
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup after remove: %v", err)
	}
	if got, want := aliasesOf(repos), []string{"repo2"}; !sameStrings(got, want) {
		t.Fatalf("backend repos after remove = %v, want %v", got, want)
	}

	if err := d.RenameGroup("backend", "api"); err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	if _, err := d.GetGroup("backend"); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("GetGroup(old) error = %v, want ErrGroupNotFound", err)
	}
	renamed, err := d.GetGroup("api")
	if err != nil {
		t.Fatalf("GetGroup(api): %v", err)
	}
	if renamed.Name != "api" || renamed.RepoCount != 1 {
		t.Fatalf("renamed group = %#v", renamed)
	}

	if err := d.DeleteGroup("api"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := d.GetGroup("api"); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("GetGroup(deleted) error = %v, want ErrGroupNotFound", err)
	}
}

func TestDefaultGroupIsProtected(t *testing.T) {
	d, _ := initDB(t)

	if _, err := d.CreateGroup(db.DefaultGroupName); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("CreateGroup(all) error = %v, want ErrReservedGroup", err)
	}
	if err := d.RenameGroup(db.DefaultGroupName, "everyone"); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("RenameGroup(all) error = %v, want ErrReservedGroup", err)
	}
	if err := d.DeleteGroup(db.DefaultGroupName); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("DeleteGroup(all) error = %v, want ErrReservedGroup", err)
	}
	if err := d.AddRepositoriesToGroup(db.DefaultGroupName, []string{"repo1"}); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("AddRepositoriesToGroup(all) error = %v, want ErrReservedGroup", err)
	}
	if err := d.RemoveRepositoriesFromGroup(db.DefaultGroupName, []string{"repo1"}); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("RemoveRepositoriesFromGroup(all) error = %v, want ErrReservedGroup", err)
	}
}

func TestGroupUnknownsReturnNotFound(t *testing.T) {
	d, _ := initDB(t)
	if _, err := d.AddRepository("repo1", "repo1", "/path/repo1", "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	if _, err := d.CreateGroup("backend"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	if err := d.AddRepositoriesToGroup("missing", []string{"repo1"}); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("AddRepositoriesToGroup missing group error = %v, want ErrGroupNotFound", err)
	}
	if err := d.AddRepositoriesToGroup("backend", []string{"ghost"}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("AddRepositoriesToGroup missing repo error = %v, want ErrNotFound", err)
	}
	if err := d.RemoveRepositoriesFromGroup("missing", []string{"repo1"}); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("RemoveRepositoriesFromGroup missing group error = %v, want ErrGroupNotFound", err)
	}
	if _, err := d.ListRepositoriesByGroup("missing"); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("ListRepositoriesByGroup missing group error = %v, want ErrGroupNotFound", err)
	}
}

func aliasesOf(repos []*db.Repository) []string {
	aliases := make([]string, 0, len(repos))
	for _, repo := range repos {
		aliases = append(aliases, repo.Alias)
	}
	return aliases
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

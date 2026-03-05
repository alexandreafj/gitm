package db_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// initDB creates a temporary SQLite database for testing.
func initDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() {
		_ = d.Close()
	})
	return d, dbPath
}

// ─── TestOpen ───────────────────────────────────────────────────────────────

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	if d == nil {
		t.Error("expected non-nil DB")
	}
}

func TestOpenCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file was not created: %v", err)
	}
}

func TestOpenIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open once
	d1, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}
	d1.Close()

	// Open again — should succeed without errors
	d2, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	d2.Close()
}

// ─── TestAddRepository ───────────────────────────────────────────────────────

func TestAddRepository(t *testing.T) {
	d, _ := initDB(t)

	repo, err := d.AddRepository("myrepo", "my-repo", "/path/to/repo", "main")
	if err != nil {
		t.Fatalf("AddRepository failed: %v", err)
	}

	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.Name != "myrepo" {
		t.Errorf("Name = %q, want %q", repo.Name, "myrepo")
	}
	if repo.Alias != "my-repo" {
		t.Errorf("Alias = %q, want %q", repo.Alias, "my-repo")
	}
	if repo.Path != "/path/to/repo" {
		t.Errorf("Path = %q, want %q", repo.Path, "/path/to/repo")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
	if repo.ID == 0 {
		t.Error("ID should be non-zero")
	}
}

func TestAddRepositoryWithEmptyAlias(t *testing.T) {
	d, _ := initDB(t)

	repo, err := d.AddRepository("myrepo", "", "/path/to/repo", "main")
	if err != nil {
		t.Fatalf("AddRepository with empty alias failed: %v", err)
	}

	// Empty alias should default to name
	if repo.Alias != "myrepo" {
		t.Errorf("Alias = %q, want %q (should default to Name)", repo.Alias, "myrepo")
	}
}

func TestAddRepositoryDuplicateAlias(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "my-repo", "/path1", "main")

	// Adding another repo with the same alias should fail
	_, err := d.AddRepository("repo2", "my-repo", "/path2", "main")
	if err == nil {
		t.Error("expected error for duplicate alias, got nil")
	}
}

func TestAddRepositoryDuplicatePath(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "alias1", "/same/path", "main")

	// Adding another repo with the same path should fail
	_, err := d.AddRepository("repo2", "alias2", "/same/path", "main")
	if err == nil {
		t.Error("expected error for duplicate path, got nil")
	}
}

// ─── TestGetRepository ───────────────────────────────────────────────────────

func TestGetRepository(t *testing.T) {
	d, _ := initDB(t)

	added, _ := d.AddRepository("myrepo", "my-repo", "/path/to/repo", "main")

	retrieved, err := d.GetRepository("my-repo")
	if err != nil {
		t.Fatalf("GetRepository failed: %v", err)
	}

	if retrieved.Alias != added.Alias {
		t.Errorf("Alias = %q, want %q", retrieved.Alias, added.Alias)
	}
	if retrieved.Path != added.Path {
		t.Errorf("Path = %q, want %q", retrieved.Path, added.Path)
	}
	if retrieved.ID != added.ID {
		t.Errorf("ID = %d, want %d", retrieved.ID, added.ID)
	}
}

func TestGetRepositoryNotFound(t *testing.T) {
	d, _ := initDB(t)

	_, err := d.GetRepository("nonexistent")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── TestGetRepositoryByPath ────────────────────────────────────────────────

func TestGetRepositoryByPath(t *testing.T) {
	d, _ := initDB(t)

	added, _ := d.AddRepository("myrepo", "my-repo", "/path/to/repo", "main")

	retrieved, err := d.GetRepositoryByPath("/path/to/repo")
	if err != nil {
		t.Fatalf("GetRepositoryByPath failed: %v", err)
	}

	if retrieved.ID != added.ID {
		t.Errorf("ID = %d, want %d", retrieved.ID, added.ID)
	}
}

func TestGetRepositoryByPathNotFound(t *testing.T) {
	d, _ := initDB(t)

	_, err := d.GetRepositoryByPath("/nonexistent/path")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── TestListRepositories ───────────────────────────────────────────────────

func TestListRepositoriesEmpty(t *testing.T) {
	d, _ := initDB(t)

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories failed: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("expected empty list, got %d repos", len(repos))
	}
}

func TestListRepositories(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "alpha", "/path1", "main")
	d.AddRepository("repo2", "beta", "/path2", "main")
	d.AddRepository("repo3", "gamma", "/path3", "main")

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories failed: %v", err)
	}

	if len(repos) != 3 {
		t.Errorf("expected 3 repos, got %d", len(repos))
	}
}

func TestListRepositoriesOrdering(t *testing.T) {
	d, _ := initDB(t)

	// Add in non-alphabetical order
	d.AddRepository("repo1", "zebra", "/path1", "main")
	d.AddRepository("repo2", "apple", "/path2", "main")
	d.AddRepository("repo3", "banana", "/path3", "main")

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories failed: %v", err)
	}

	// Should be ordered by alias
	expected := []string{"apple", "banana", "zebra"}
	for i, exp := range expected {
		if repos[i].Alias != exp {
			t.Errorf("repos[%d].Alias = %q, want %q", i, repos[i].Alias, exp)
		}
	}
}

// ─── TestRemoveRepository ───────────────────────────────────────────────────

func TestRemoveRepository(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "my-repo", "/path/to/repo", "main")

	err := d.RemoveRepository("my-repo")
	if err != nil {
		t.Fatalf("RemoveRepository failed: %v", err)
	}

	// Verify it's gone
	_, err = d.GetRepository("my-repo")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected repo to be removed, but found it or got unexpected error: %v", err)
	}
}

func TestRemoveRepositoryNotFound(t *testing.T) {
	d, _ := initDB(t)

	err := d.RemoveRepository("nonexistent")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── TestRenameRepository ───────────────────────────────────────────────────

func TestRenameRepository(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "old-alias", "/path/to/repo", "main")

	err := d.RenameRepository("old-alias", "new-alias")
	if err != nil {
		t.Fatalf("RenameRepository failed: %v", err)
	}

	// Verify the old alias is gone
	_, err = d.GetRepository("old-alias")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected old alias to not exist, got %v", err)
	}

	// Verify the new alias works
	repo, err := d.GetRepository("new-alias")
	if err != nil {
		t.Errorf("expected to find repo with new alias, got %v", err)
	}
	if repo.Alias != "new-alias" {
		t.Errorf("Alias = %q, want %q", repo.Alias, "new-alias")
	}
}

func TestRenameRepositoryNotFound(t *testing.T) {
	d, _ := initDB(t)

	err := d.RenameRepository("nonexistent", "new-name")
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ─── TestUpdateDefaultBranch ────────────────────────────────────────────────

func TestUpdateDefaultBranch(t *testing.T) {
	d, _ := initDB(t)

	d.AddRepository("repo1", "my-repo", "/path/to/repo", "main")

	err := d.UpdateDefaultBranch("my-repo", "master")
	if err != nil {
		t.Fatalf("UpdateDefaultBranch failed: %v", err)
	}

	// Verify the change
	repo, _ := d.GetRepository("my-repo")
	if repo.DefaultBranch != "master" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "master")
	}
}

// ─── TestClose ───────────────────────────────────────────────────────────────

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, _ := db.Open(dbPath)

	err := d.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

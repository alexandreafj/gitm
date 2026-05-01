package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUpdate_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runUpdate(nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

func TestRunUpdate_SkipsDirtyTracked(t *testing.T) {
	database = setupTestDB(t)
	repo, dir := newRepo(t, database, "repo1")

	writeFile(t, dir, "README.md", "changed\n")

	if err := runUpdate(nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	_ = repo
}

func TestRunUpdate_Pulls(t *testing.T) {
	database = setupTestDB(t)
	repo, origin, _ := initRepoWithRemote(t)

	_, err := database.AddRepository("repo1", "repo1", repo, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	clone := cloneRepo(t, origin)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	writeFile(t, clone, "from-remote.txt", "remote\n")
	mustRunGit(t, clone, "add", "from-remote.txt")
	mustRunGit(t, clone, "commit", "-m", "remote change")
	mustRunGit(t, clone, "push")

	if err := runUpdate(nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

func TestRunUpdate_RepoFlag_SingleRepo(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir, origin1, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir, origin2, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	pushRemoteChange(t, origin1, "from-remote1.txt")
	pushRemoteChange(t, origin2, "from-remote2.txt")

	if err := runUpdate([]string{"repo1"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo1Dir, "from-remote1.txt")); err != nil {
		t.Fatal("expected from-remote1.txt in repo1 after update")
	}
	if _, err := os.Stat(filepath.Join(repo2Dir, "from-remote2.txt")); err == nil {
		t.Fatal("repo2 should not have been updated")
	}
}

func TestRunUpdate_RepoFlag_MultipleRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir, origin1, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir, origin2, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	repo3Dir, origin3, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo3", "repo3", repo3Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	pushRemoteChange(t, origin1, "from-remote1.txt")
	pushRemoteChange(t, origin2, "from-remote2.txt")
	pushRemoteChange(t, origin3, "from-remote3.txt")

	if err := runUpdate([]string{"repo1", "repo3"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo1Dir, "from-remote1.txt")); err != nil {
		t.Fatal("expected from-remote1.txt in repo1")
	}
	if _, err := os.Stat(filepath.Join(repo2Dir, "from-remote2.txt")); err == nil {
		t.Fatal("repo2 should not have been updated")
	}
	if _, err := os.Stat(filepath.Join(repo3Dir, "from-remote3.txt")); err != nil {
		t.Fatal("expected from-remote3.txt in repo3")
	}
}

func TestRunUpdate_RepoFlag_UnknownAlias(t *testing.T) {
	database = setupTestDB(t)
	_, _ = newRepo(t, database, "repo1")

	err := runUpdate([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdate_RepoFlag_EmptySlice(t *testing.T) {
	database = setupTestDB(t)

	if err := runUpdate([]string{}); err != nil {
		t.Fatalf("runUpdate with empty slice: %v", err)
	}
}

func TestRunUpdate_ReturnsErrorOnFailure(t *testing.T) {
	// Tests Finding #4: runUpdate should return a non-nil error when repos fail.
	database = setupTestDB(t)

	// Register a repo with a non-existent path — git operations will fail.
	if _, err := database.AddRepository("broken", "broken", "/nonexistent/path/broken", "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runUpdate(nil)
	if err == nil {
		t.Fatal("expected error when repo git operations fail, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update") {
		t.Errorf("error = %q, want to contain \"failed to update\"", err.Error())
	}
}

func pushRemoteChange(t *testing.T, origin, filename string) {
	t.Helper()
	clone := cloneRepo(t, origin)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	writeFile(t, clone, filename, "remote content\n")
	mustRunGit(t, clone, "add", filename)
	mustRunGit(t, clone, "commit", "-m", "add "+filename)
	mustRunGit(t, clone, "push")
}

package cli

import (
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestReconcileDefaultBranchesUpdatesMemoryAndDatabase(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "stale", "main", "master")

	if err := reconcileDefaultBranches(database, []*db.Repository{repo}, true); err != nil {
		t.Fatalf("reconcileDefaultBranches: %v", err)
	}

	if repo.DefaultBranch != "master" {
		t.Fatalf("in-memory default branch = %q, want master", repo.DefaultBranch)
	}
	stored, err := database.GetRepository(repo.Alias)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master", stored.DefaultBranch)
	}
}

func TestReconcileDefaultBranchesWarnsOnceAndPreservesCachedValues(t *testing.T) {
	database = setupTestDB(t)
	zeta, _ := newRepo(t, database, "zeta")
	alpha, _ := newRepo(t, database, "alpha")

	output := captureOutput(t, func() {
		if err := reconcileDefaultBranches(database, []*db.Repository{zeta, alpha}, true); err != nil {
			t.Fatalf("reconcileDefaultBranches: %v", err)
		}
	})

	if zeta.DefaultBranch != "main" || alpha.DefaultBranch != "main" {
		t.Fatalf("cached values changed after lookup failures: zeta=%q alpha=%q", zeta.DefaultBranch, alpha.DefaultBranch)
	}
	if count := strings.Count(output, "Warning:"); count != 1 {
		t.Fatalf("warning count = %d, want 1; output:\n%s", count, output)
	}
	if !strings.Contains(output, "alpha, zeta") {
		t.Fatalf("warning aliases are not deterministic: %s", output)
	}
}

func TestReconcileDefaultBranchesDryRunUpdatesMemoryWithoutPersistence(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "dry", "main", "master")

	if err := reconcileDefaultBranches(database, []*db.Repository{repo}, false); err != nil {
		t.Fatalf("reconcileDefaultBranches: %v", err)
	}

	if repo.DefaultBranch != "master" {
		t.Fatalf("in-memory default branch = %q, want master", repo.DefaultBranch)
	}
	stored, err := database.GetRepository(repo.Alias)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("stored default branch = %q, want stale main during dry-run", stored.DefaultBranch)
	}
}

func TestReconcileDefaultBranchesReturnsContextualPersistenceError(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "broken-cache", "main", "master")
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := reconcileDefaultBranches(database, []*db.Repository{repo}, true)
	if err == nil {
		t.Fatal("expected persistence error")
	}
	if !strings.Contains(err.Error(), "broken-cache") || !strings.Contains(err.Error(), "persist default branch") {
		t.Fatalf("error = %q, want persistence context and alias", err)
	}
	if repo.DefaultBranch != "master" {
		t.Fatalf("in-memory default branch = %q, want master even when persistence fails", repo.DefaultBranch)
	}
}

func addRepoWithRemoteDefault(t *testing.T, alias, cached, remoteDefault string) *db.Repository {
	t.Helper()
	repoDir, originDir, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "checkout", "-b", remoteDefault)
	mustRunGit(t, repoDir, "push", "--set-upstream", "origin", remoteDefault)
	mustRunGit(t, originDir, "symbolic-ref", "HEAD", "refs/heads/"+remoteDefault)
	repo, err := database.AddRepository(alias, alias, repoDir, cached)
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	return repo
}

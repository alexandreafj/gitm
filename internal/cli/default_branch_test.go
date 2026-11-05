package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func TestReconcileDefaultBranchesUsesLocalOriginHeadWithoutNetwork(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "fast", "master", "master")
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "master")
	mustRunGit(t, repo.Path, "remote", "set-url", "origin", "/nonexistent")

	output := captureOutput(t, func() {
		if err := reconcileDefaultBranches(database, []*db.Repository{repo}, true); err != nil {
			t.Fatalf("reconcileDefaultBranches: %v", err)
		}
	})

	if strings.Contains(output, "Warning:") {
		t.Fatalf("origin/HEAD matching the cache should not need the remote; output:\n%s", output)
	}
	if repo.DefaultBranch != "master" {
		t.Fatalf("in-memory default branch = %q, want master", repo.DefaultBranch)
	}
}

func TestReconcileDefaultBranchesRepairsLocalHeadThatDisagreesWithCache(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "disagree", "master", "master")
	// Git creates origin/HEAD on clone or fetch but never updates it, so it can
	// still name main after origin moved to master.
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "main")

	if err := reconcileDefaultBranches(database, []*db.Repository{repo}, true); err != nil {
		t.Fatalf("reconcileDefaultBranches: %v", err)
	}

	stored, err := database.GetRepository(repo.Alias)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" || repo.DefaultBranch != "master" {
		t.Fatalf("default branch = stored %q, in-memory %q; want master kept", stored.DefaultBranch, repo.DefaultBranch)
	}
	if got := mustRunGit(t, repo.Path, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/master" {
		t.Fatalf("origin/HEAD = %q, want refs/remotes/origin/master", got)
	}
}

func TestReconcileDefaultBranchesOfflineDisagreementKeepsCache(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "offline", "master", "master")
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "main")
	mustRunGit(t, repo.Path, "remote", "set-url", "origin", "/nonexistent")

	output := captureOutput(t, func() {
		if err := reconcileDefaultBranches(database, []*db.Repository{repo}, true); err != nil {
			t.Fatalf("reconcileDefaultBranches: %v", err)
		}
	})

	if count := strings.Count(output, "Warning:"); count != 1 {
		t.Fatalf("warning count = %d, want 1; output:\n%s", count, output)
	}
	stored, err := database.GetRepository(repo.Alias)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" || repo.DefaultBranch != "master" {
		t.Fatalf("default branch = stored %q, in-memory %q; want cached master", stored.DefaultBranch, repo.DefaultBranch)
	}
}

func TestReconcileDefaultBranchesFallsBackToRemoteWhenLocalHeadDangles(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "dangling", "main", "master")
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "main")
	mustRunGit(t, repo.Path, "update-ref", "-d", "refs/remotes/origin/main")

	if err := reconcileDefaultBranches(database, []*db.Repository{repo}, true); err != nil {
		t.Fatalf("reconcileDefaultBranches: %v", err)
	}

	stored, err := database.GetRepository(repo.Alias)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master from origin", stored.DefaultBranch)
	}
	if got := mustRunGit(t, repo.Path, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/master" {
		t.Fatalf("origin/HEAD = %q, want refs/remotes/origin/master", got)
	}
}

func TestReconcileDefaultBranchesFallsBackToRemoteWhenLocalHeadMissing(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "stale", "main", "master")
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "--delete")

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
	local, err := git.LocalDefaultBranch(repo.Path)
	if err != nil {
		t.Fatalf("LocalDefaultBranch: %v", err)
	}
	if local != "master" {
		t.Fatalf("local origin/HEAD = %q, want master written after the remote lookup", local)
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
	mustRunGit(t, repo.Path, "remote", "set-head", "origin", "--delete")

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
	if local, err := git.LocalDefaultBranch(repo.Path); err != nil || local != "" {
		t.Fatalf("dry-run wrote origin/HEAD: LocalDefaultBranch = %q, %v; want unset", local, err)
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

func TestReconcileDefaultBranchesReportsRemovedAlias(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "removed-cache", "main", "master")
	if err := database.RemoveRepository(repo.Alias); err != nil {
		t.Fatalf("RemoveRepository: %v", err)
	}

	err := reconcileDefaultBranches(database, []*db.Repository{repo}, true)
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("reconcileDefaultBranches() error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "persist default branch for removed-cache") {
		t.Fatalf("reconcileDefaultBranches() error = %q, want removed alias context", err)
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

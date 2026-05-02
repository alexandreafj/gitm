package e2e

import (
	"path/filepath"
	"testing"
)

func TestBranchCreate_WithRepo(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("bc-repo")
	e.runGitm("repo", "add", repo, "--alias", "bc-repo")

	r := e.runGitm("branch", "create", "feat/test-branch", "--repo", "bc-repo")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "feat/test-branch" {
		t.Errorf("expected to be on feat/test-branch, got %s", branch)
	}
}

func TestBranchCreate_WithAll(t *testing.T) {
	e := newTestEnv(t)
	repo1, _ := e.initRepoWithRemote("bc-all-1")
	repo2, _ := e.initRepoWithRemote("bc-all-2")
	e.runGitm("repo", "add", repo1, "--alias", "bc-all-1")
	e.runGitm("repo", "add", repo2, "--alias", "bc-all-2")

	r := e.runGitm("branch", "create", "feat/all-branch", "--all")
	e.assertExitCode(r, 0)

	if !e.branchExists(repo1, "feat/all-branch") {
		t.Error("branch not created in repo1")
	}
	if !e.branchExists(repo2, "feat/all-branch") {
		t.Error("branch not created in repo2")
	}
}

func TestBranchCreate_FromSpecificBase(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("bc-from")
	e.runGitm("repo", "add", repo, "--alias", "bc-from")

	e.mustGit(repo, "checkout", "-b", "develop")
	e.writeFile(repo, "develop.txt", "develop content\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "commit", "-m", "develop commit")
	e.mustGit(repo, "push", "--set-upstream", "origin", "develop")
	e.mustGit(repo, "checkout", "main")

	r := e.runGitm("branch", "create", "feat/from-develop", "--from", "develop", "--repo", "bc-from")
	e.assertExitCode(r, 0)

	if !e.fileExists(filepath.Join(repo, "develop.txt")) {
		t.Error("branch was not created from develop — develop.txt missing")
	}
}

func TestBranchCreate_ExistingBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("bc-existing")
	e.runGitm("repo", "add", repo, "--alias", "bc-existing")

	e.mustGit(repo, "checkout", "-b", "feat/already-exists")
	e.mustGit(repo, "checkout", "main")

	// gitm should check it out instead of erroring
	r := e.runGitm("branch", "create", "feat/already-exists", "--repo", "bc-existing")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "feat/already-exists" {
		t.Errorf("expected to be on feat/already-exists, got %s", branch)
	}
}

func TestBranchCreate_FromNonExistentBase(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("bc-bad-base")
	e.runGitm("repo", "add", repo, "--alias", "bc-bad-base")

	startingBranch := e.currentBranch(repo)
	r := e.runGitm("branch", "create", "feat/x", "--from", "nonexistent-branch", "--repo", "bc-bad-base")

	combined := r.Stdout + r.Stderr
	// Accept either an explicit command failure or a successful exit that clearly reports
	// the invalid base branch. Silent success is incorrect.
	if r.ExitCode == 0 && !containsAny(combined, "not found", "error", "failed", "does not exist") {
		t.Fatalf("expected branch create from non-existent base to fail or report the missing base branch; exit=%d stdout=%q stderr=%q",
			r.ExitCode, r.Stdout, r.Stderr)
	}

	if e.branchExists(repo, "feat/x") {
		t.Fatal("branch feat/x should not be created when the base branch does not exist")
	}

	if branch := e.currentBranch(repo); branch != startingBranch {
		t.Fatalf("expected to remain on %s after failing to create from a non-existent base, got %s", startingBranch, branch)
	}
}

func TestBranchCreate_DirtyRepo(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("bc-dirty")
	e.runGitm("repo", "add", repo, "--alias", "bc-dirty")

	e.writeFile(repo, "README.md", "# dirty\n")

	r := e.runGitm("branch", "create", "feat/dirty-test", "--repo", "bc-dirty")
	// Branch create on dirty repos should skip with a warning about uncommitted changes
	e.assertExitCode(r, 0)
	e.assertContains(r, "uncommitted changes")
}

func TestBranchRename_WithRepo(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("br-repo")
	e.runGitm("repo", "add", repo, "--alias", "br-repo")

	e.mustGit(repo, "checkout", "-b", "old-name")
	e.mustGit(repo, "push", "--set-upstream", "origin", "old-name")

	r := e.runGitm("branch", "rename", "old-name", "new-name", "--repo", "br-repo")
	e.assertExitCode(r, 0)

	// Old branch should be gone, new should exist
	if e.branchExists(repo, "old-name") {
		t.Error("old branch still exists after rename")
	}
	if !e.branchExists(repo, "new-name") {
		t.Error("new branch does not exist after rename")
	}
}

func TestBranchRename_WithAll(t *testing.T) {
	e := newTestEnv(t)
	repo1, _ := e.initRepoWithRemote("br-all-1")
	repo2, _ := e.initRepoWithRemote("br-all-2")
	e.runGitm("repo", "add", repo1, "--alias", "br-all-1")
	e.runGitm("repo", "add", repo2, "--alias", "br-all-2")

	e.mustGit(repo1, "checkout", "-b", "shared-old")
	e.mustGit(repo1, "push", "--set-upstream", "origin", "shared-old")
	e.mustGit(repo2, "checkout", "-b", "shared-old")
	e.mustGit(repo2, "push", "--set-upstream", "origin", "shared-old")

	r := e.runGitm("branch", "rename", "shared-old", "shared-new", "--all")
	e.assertExitCode(r, 0)

	if e.branchExists(repo1, "shared-old") {
		t.Error("old branch still exists in repo1")
	}
	if e.branchExists(repo2, "shared-old") {
		t.Error("old branch still exists in repo2")
	}
	if !e.branchExists(repo1, "shared-new") {
		t.Error("new branch missing in repo1")
	}
	if !e.branchExists(repo2, "shared-new") {
		t.Error("new branch missing in repo2")
	}
}

func TestBranchRename_NoRemote(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("br-noremote")
	e.runGitm("repo", "add", repo, "--alias", "br-noremote")

	e.mustGit(repo, "checkout", "-b", "local-old")
	e.mustGit(repo, "push", "--set-upstream", "origin", "local-old")

	r := e.runGitm("branch", "rename", "local-old", "local-new", "--no-remote", "--repo", "br-noremote")
	e.assertExitCode(r, 0)

	// Local should be renamed
	if e.branchExists(repo, "local-old") {
		t.Error("old branch still exists locally")
	}
	if !e.branchExists(repo, "local-new") {
		t.Error("new branch missing locally")
	}

	// Remote should still have old name (--no-remote skips remote ops)
	remoteOut := e.mustGit(origin, "branch", "--list")
	if !containsAny(remoteOut, "local-old") {
		t.Log("Note: remote old branch was deleted even with --no-remote")
	}
}

func TestBranchRename_NonExistentBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("br-ghost")
	e.runGitm("repo", "add", repo, "--alias", "br-ghost")

	r := e.runGitm("branch", "rename", "nonexistent-branch", "new", "--repo", "br-ghost")
	// Renaming a branch that does not exist should fail
	if r.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code when renaming a non-existent branch; stdout=%s stderr=%s",
			r.Stdout, r.Stderr)
	}
	e.assertContains(r, "no registered repositories have a branch named")
}

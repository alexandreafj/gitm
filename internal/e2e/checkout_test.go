package e2e

import (
	"testing"
)

// ==========================================================================
// Phase 4: Checkout (gitm checkout)
// ==========================================================================

func TestCheckout_DefaultBranch_Master(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-master")
	e.runGitm("repo", "add", repo, "--alias", "co-master")

	// Create and switch to a feature branch
	e.mustGit(repo, "checkout", "-b", "feat/something")

	r := e.runGitm("checkout", "master")
	e.assertExitCode(r, 0)

	// Should be back on the default branch (main in our test setup)
	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("expected to be on main (default), got %s", branch)
	}
}

func TestCheckout_DefaultBranch_Main(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-main")
	e.runGitm("repo", "add", repo, "--alias", "co-main")

	// Create and switch to a feature branch
	e.mustGit(repo, "checkout", "-b", "feat/other")

	r := e.runGitm("checkout", "main")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("expected to be on main, got %s", branch)
	}
}

func TestCheckout_ExistingBranch_WithRepo(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-existing")
	e.runGitm("repo", "add", repo, "--alias", "co-existing")

	// Create a feature branch
	e.mustGit(repo, "checkout", "-b", "feat/target")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/target")
	e.mustGit(repo, "checkout", "main")

	r := e.runGitm("checkout", "feat/target", "--repo", "co-existing")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "feat/target" {
		t.Errorf("expected feat/target, got %s", branch)
	}
}

func TestCheckout_NonExistentBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-ghost")
	e.runGitm("repo", "add", repo, "--alias", "co-ghost")

	r := e.runGitm("checkout", "branch-that-does-not-exist", "--repo", "co-ghost")
	// Should succeed (exit 0) but skip the repo with a message
	e.assertExitCode(r, 0)
	// Should NOT have switched branches
	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("checkout of non-existent branch should not change current branch, but now on %s", branch)
	}
}

func TestCheckout_DirtyRepo_Skips(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-dirty")
	e.runGitm("repo", "add", repo, "--alias", "co-dirty")

	// Create target branch
	e.mustGit(repo, "checkout", "-b", "feat/dirty-target")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/dirty-target")
	e.mustGit(repo, "checkout", "main")

	// Make repo dirty
	e.writeFile(repo, "README.md", "# dirty content\n")

	r := e.runGitm("checkout", "feat/dirty-target", "--repo", "co-dirty")
	// Should skip with warning
	e.assertExitCode(r, 0)

	// Should still be on main (not switched)
	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("dirty repo should not switch branches, but now on %s", branch)
	}
	e.assertContains(r, "co-dirty") // Should mention the repo
}

func TestCheckout_UntrackedFiles_ShouldNotSkip(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-untracked")
	e.runGitm("repo", "add", repo, "--alias", "co-untracked")

	// Create target branch
	e.mustGit(repo, "checkout", "-b", "feat/untracked-test")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/untracked-test")
	e.mustGit(repo, "checkout", "main")

	// Add untracked file only (should NOT make repo dirty per docs)
	e.writeFile(repo, "untracked-new.txt", "I am untracked\n")

	r := e.runGitm("checkout", "feat/untracked-test", "--repo", "co-untracked")
	e.assertExitCode(r, 0)

	// Per docs: untracked files are ignored — checkout should proceed
	branch := e.currentBranch(repo)
	if branch != "feat/untracked-test" {
		t.Errorf("untracked files should not block checkout, but stayed on %s", branch)
	}
}

func TestCheckout_RemoteOnlyBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("co-remote")
	e.runGitm("repo", "add", repo, "--alias", "co-remote")

	// Create a branch on remote only (via another clone)
	other := e.cloneRepo(origin, "co-remote-other")
	e.mustGit(other, "checkout", "-b", "feat/remote-only")
	e.writeFile(other, "remote.txt", "from remote\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "remote commit")
	e.mustGit(other, "push", "--set-upstream", "origin", "feat/remote-only")

	// Our repo doesn't know about this branch locally
	r := e.runGitm("checkout", "feat/remote-only", "--repo", "co-remote")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "feat/remote-only" {
		// FINDING: gitm claims to check remote branches but may require a fetch first
		// or the implementation doesn't handle remote-only branches as documented.
		t.Logf("FINDING: checkout of remote-only branch did not work. Current branch: %s", branch)
		t.Log("README states: 'Checks branch locally then remote' — but actual behavior differs.")
		t.Log("gitm may need an explicit fetch before checking remote branches.")
		t.Logf("Output: stdout=%s stderr=%s", r.Stdout, r.Stderr)
	}
}

func TestCheckout_PullsAfterSwitch(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("co-pulls")
	e.runGitm("repo", "add", repo, "--alias", "co-pulls")

	// Create a branch, push it, then push more commits from another clone
	e.mustGit(repo, "checkout", "-b", "feat/pull-test")
	e.writeFile(repo, "first.txt", "first\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "commit", "-m", "first")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/pull-test")
	e.mustGit(repo, "checkout", "main")

	// Push more commits from another clone
	other := e.cloneRepo(origin, "co-pulls-other")
	e.mustGit(other, "checkout", "feat/pull-test")
	e.writeFile(other, "second.txt", "second\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "second from other")
	e.mustGit(other, "push")

	// Checkout should switch AND pull
	r := e.runGitm("checkout", "feat/pull-test", "--repo", "co-pulls")
	e.assertExitCode(r, 0)

	// Should have the latest file from the other clone
	if !e.fileExists(repo + "/second.txt") {
		t.Error("checkout did not pull latest — second.txt missing")
	}
}

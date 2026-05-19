package e2e

import (
	"path/filepath"
	"testing"
)

func TestCheckout_DefaultBranch_Master(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-master")
	e.runGitm("repo", "add", repo, "--alias", "co-master")

	e.mustGit(repo, "checkout", "-b", "feat/something")

	r := e.runGitm("checkout", "master")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("expected to be on main (default), got %s", branch)
	}
}

func TestCheckout_DefaultBranch_Main(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-main")
	e.runGitm("repo", "add", repo, "--alias", "co-main")

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
	e.assertExitCode(r, 0)
	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("checkout of non-existent branch should not change current branch, but now on %s", branch)
	}
}

func TestCheckout_DirtyRepo_NonConflicting_Succeeds(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-dirty-ok")
	e.runGitm("repo", "add", repo, "--alias", "co-dirty-ok")

	e.mustGit(repo, "checkout", "-b", "feat/dirty-target")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/dirty-target")
	e.mustGit(repo, "checkout", "main")

	e.writeFile(repo, "README.md", "# dirty content\n")

	r := e.runGitm("checkout", "feat/dirty-target", "--repo", "co-dirty-ok")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "feat/dirty-target" {
		t.Errorf("non-conflicting dirty repo should switch branches, but stayed on %s", branch)
	}
}

func TestCheckout_DirtyRepo_Conflicting_Skips(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-dirty-conflict")
	e.runGitm("repo", "add", repo, "--alias", "co-dirty-conflict")

	e.writeFile(repo, "conflict.txt", "main-version\n")
	e.mustGit(repo, "add", "conflict.txt")
	e.mustGit(repo, "commit", "-m", "add conflict file on main")
	e.mustGit(repo, "push")

	e.mustGit(repo, "checkout", "-b", "feat/conflict-target")
	e.writeFile(repo, "conflict.txt", "feature-version\n")
	e.mustGit(repo, "add", "conflict.txt")
	e.mustGit(repo, "commit", "-m", "change conflict file on feature")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/conflict-target")
	e.mustGit(repo, "checkout", "main")

	e.writeFile(repo, "conflict.txt", "local dirty change\n")

	r := e.runGitm("checkout", "feat/conflict-target", "--repo", "co-dirty-conflict")
	e.assertExitCode(r, 0)

	branch := e.currentBranch(repo)
	if branch != "main" {
		t.Errorf("conflicting dirty repo should stay on current branch, but switched to %s", branch)
	}
	e.assertContains(r, "co-dirty-conflict")
}

func TestCheckout_UntrackedFiles_ShouldNotSkip(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("co-untracked")
	e.runGitm("repo", "add", repo, "--alias", "co-untracked")

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
		t.Fatalf("expected checkout of remote-only branch to switch to %q, got %q\nstdout: %s\nstderr: %s",
			"feat/remote-only", branch, r.Stdout, r.Stderr)
	}
}

func TestCheckout_PullsAfterSwitch(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("co-pulls")
	e.runGitm("repo", "add", repo, "--alias", "co-pulls")

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

	if !e.fileExists(filepath.Join(repo, "second.txt")) {
		t.Error("checkout did not pull latest — second.txt missing")
	}
}

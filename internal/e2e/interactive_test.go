package e2e

import (
	"testing"
)

// Note: Commit is heavily TUI-dependent (file selection + message input).
// We can only test edge cases that exit before TUI interaction.

func TestCommit_NoDirtyRepos(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("commit-clean")
	e.runGitm("repo", "add", repo, "--alias", "commit-clean")

	r := e.runGitm("commit", "--repo", "commit-clean")
	e.assertExitCode(r, 0)
	e.assertContains(r, "No dirty repositories found")
}

func TestCommit_ProtectedDefaultBranch(t *testing.T) {
	// Commit on a protected default branch requires TTY interaction for confirmation;
	// behavior is non-deterministic in non-TTY environments.
	t.Skip("commit protection behavior is non-deterministic in non-TTY environments")
}

// Note: Discard is TUI-dependent for file selection.
// We can only test the "all clean" edge case automatically.

func TestDiscard_AllReposClean(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("discard-clean")
	e.runGitm("repo", "add", repo, "--alias", "discard-clean")

	r := e.runGitm("discard", "--repo", "discard-clean")
	e.assertExitCode(r, 0)
	e.assertContains(r, "clean")
}

// Note: stash has no --repo flag, fully TUI-dependent.
// We test stash list (non-interactive) and edge cases.

func TestStashList_NoStashes(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("stash-empty")
	e.runGitm("repo", "add", repo, "--alias", "stash-empty")

	r := e.runGitm("stash", "list")
	e.assertExitCode(r, 0)
	e.assertContains(r, "No repositories have stash entries")
}

func TestStashList_WithStashes(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("stash-has")
	e.runGitm("repo", "add", repo, "--alias", "stash-has")

	e.writeFile(repo, "stashme.txt", "stash this\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "stash", "push", "-m", "manual stash for test")

	r := e.runGitm("stash", "list")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "stash-has")
}

// Note: Reset is TUI-dependent (repo selection). No --repo flag.
// We document expectations but cannot fully automate.

func TestReset_NoReposToReset(t *testing.T) {
	t.Skip("reset requires TTY for repo selection — cannot test non-interactively")
}

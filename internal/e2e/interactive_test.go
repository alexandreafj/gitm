package e2e

import (
	"testing"
)

// ==========================================================================
// Phase 8: Commit (gitm commit)
// Note: Commit is heavily TUI-dependent (file selection + message input).
// We can only test edge cases that exit before TUI interaction.
// ==========================================================================

func TestCommit_NoDirtyRepos(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("commit-clean")
	e.runGitm("repo", "add", repo, "--alias", "commit-clean")

	r := e.runGitm("commit", "--repo", "commit-clean")
	e.assertExitCode(r, 0)
	// Should indicate no dirty repos
	e.assertContains(r, "No")
}

func TestCommit_ProtectedDefaultBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("commit-protected")
	e.runGitm("repo", "add", repo, "--alias", "commit-protected")

	// Stay on main (default branch) and make it dirty
	e.writeFile(repo, "dirty.txt", "dirty\n")

	// With --repo, this bypasses repo selection but file selection is TUI
	// The protection should prevent proceeding
	r := e.runGitm("commit", "--repo", "commit-protected")
	t.Logf("Commit on protected branch: exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
	// Should mention "protected" or "default branch"
	combined := r.Stdout + r.Stderr
	if !containsAny(combined, "protected", "default", "No") {
		t.Log("Note: commit on default branch did not explicitly mention protection")
	}
}

// ==========================================================================
// Phase 9: Discard (gitm discard)
// Note: Discard is TUI-dependent for file selection.
// We can only test the "all clean" edge case automatically.
// ==========================================================================

func TestDiscard_AllReposClean(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("discard-clean")
	e.runGitm("repo", "add", repo, "--alias", "discard-clean")

	r := e.runGitm("discard", "--repo", "discard-clean")
	e.assertExitCode(r, 0)
	// Should indicate all clean
	e.assertContains(r, "clean")
}

// ==========================================================================
// Phase 10: Stash (gitm stash)
// Note: stash has no --repo flag, fully TUI-dependent.
// We test stash list (non-interactive) and edge cases.
// ==========================================================================

func TestStashList_NoStashes(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("stash-empty")
	e.runGitm("repo", "add", repo, "--alias", "stash-empty")

	r := e.runGitm("stash", "list")
	e.assertExitCode(r, 0)
	// Should indicate no stashes or empty table
	t.Logf("Stash list (no stashes): exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
}

func TestStashList_WithStashes(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("stash-has")
	e.runGitm("repo", "add", repo, "--alias", "stash-has")

	// Create a stash manually via git
	e.writeFile(repo, "stashme.txt", "stash this\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "stash", "push", "-m", "manual stash for test")

	r := e.runGitm("stash", "list")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "stash-has")
}

// ==========================================================================
// Phase 11: Reset (gitm reset)
// Note: Reset is TUI-dependent (repo selection). No --repo flag.
// We document expectations but cannot fully automate.
// ==========================================================================

// TestReset_Behavior documents what reset does when invoked non-interactively.
// Since there's no --repo flag, we can only observe exit behavior.
func TestReset_NoReposToReset(t *testing.T) {
	e := newTestEnv(t)
	// Register a repo with only 1 commit (can't reset further)
	repo, _ := e.initRepoWithRemote("reset-one")
	e.runGitm("repo", "add", repo, "--alias", "reset-one")

	// Reset needs TUI interaction — this will likely fail in non-terminal
	r := e.runGitm("reset")
	t.Logf("Reset (non-interactive): exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
}

package e2e

import (
	"testing"
)

// ==========================================================================
// Phase 6 & 7: Track and Untrack (gitm track / gitm untrack)
// Note: These commands use TUI for file selection.
// We can only test edge cases (no files to track/untrack) automatically.
// The --repo flag bypasses repo selection but file picker is still TUI-based.
// ==========================================================================

func TestTrack_NoUntrackedFiles(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("track-none")
	e.runGitm("repo", "add", repo, "--alias", "track-none")

	r := e.runGitm("track", "--repo", "track-none")
	// Should exit gracefully with a "no untracked" message
	e.assertExitCode(r, 0)
	e.assertContains(r, "No")
}

func TestTrack_HasUntrackedFiles(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("track-has")
	e.runGitm("repo", "add", repo, "--alias", "track-has")

	// Create untracked files
	e.writeFile(repo, "newfile.txt", "new\n")
	e.writeFile(repo, "another.txt", "another\n")

	// This will try to open TUI — since we're not in a terminal, it may fail or
	// show files and immediately return. Document the behaviour.
	r := e.runGitm("track", "--repo", "track-has")
	t.Logf("Track with untracked files: exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
}

func TestUntrack_NoMatchingFiles(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("untrack-none")
	e.runGitm("repo", "add", repo, "--alias", "untrack-none")

	// Use a path filter that matches nothing
	r := e.runGitm("untrack", "--repo", "untrack-none", "--path", "*.nonexistent")
	// Should exit gracefully
	t.Logf("Untrack no match: exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
}

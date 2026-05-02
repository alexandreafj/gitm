package e2e

import (
	"testing"
)

// Note: These commands use TUI for file selection.
// We can only test edge cases (no files to track/untrack) automatically.
// The --repo flag bypasses repo selection but file picker is still TUI-based.

func TestTrack_NoUntrackedFiles(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("track-none")
	e.runGitm("repo", "add", repo, "--alias", "track-none")

	r := e.runGitm("track", "--repo", "track-none")
	e.assertExitCode(r, 0)
	e.assertContains(r, "No untracked files found")
}

func TestTrack_HasUntrackedFiles(t *testing.T) {
	// Track file picker requires TTY interaction — cannot test non-interactively.
	t.Skip("track file picker requires TTY interaction")
}

func TestUntrack_NoMatchingFiles(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("untrack-none")
	e.runGitm("repo", "add", repo, "--alias", "untrack-none")

	r := e.runGitm("untrack", "--repo", "untrack-none", "--path", "*.nonexistent")
	e.assertExitCode(r, 0)
	e.assertContains(r, "No files matching")
}

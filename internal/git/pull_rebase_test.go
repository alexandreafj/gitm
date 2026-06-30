package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

// advanceOriginMain pushes a new commit to origin's main branch by cloning the
// bare origin, committing, and pushing — simulating the remote moving ahead
// while a local clone is unaware.
func advanceOriginMain(t *testing.T, originDir, filename, content string) {
	t.Helper()
	clone := cloneRepo(t, originDir)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	mustRunGit(t, clone, "config", "commit.gpgsign", "false")
	writeFile(t, clone, filename, content)
	mustRunGit(t, clone, "add", ".")
	mustRunGit(t, clone, "commit", "-m", "advance main: "+filename)
	mustRunGit(t, clone, "push", "origin", "main")
}

func TestPullRebase_RebasesDivergedBranchOntoRemote(t *testing.T) {
	workDir, originDir := initRepoWithRemote(t)

	// Remote advances with remote.txt; local advances with local.txt — the
	// branch has now diverged (local is both ahead and behind origin/main).
	advanceOriginMain(t, originDir, "remote.txt", "remote\n")
	makeCommit(t, workDir, "local.txt", "local\n", "local commit")

	if _, err := git.PullRebase(workDir, "main"); err != nil {
		t.Fatalf("PullRebase: %v", err)
	}

	// Both the remote commit and the rebased local commit must be present.
	for _, f := range []string{"remote.txt", "local.txt"} {
		if _, err := os.Stat(filepath.Join(workDir, f)); err != nil {
			t.Errorf("expected %s after rebase: %v", f, err)
		}
	}

	if files, err := git.UnmergedFiles(workDir); err != nil || len(files) != 0 {
		t.Fatalf("expected no conflicts after clean rebase, got %v (err %v)", files, err)
	}

	// History is linear (rebase, not merge): HEAD has exactly one parent, so the
	// "<commit> <parent>" rev-list line has two fields, not three.
	if parents := mustRunGit(t, workDir, "rev-list", "--parents", "-n", "1", "HEAD"); len(strings.Fields(parents)) != 2 {
		t.Errorf("expected a single-parent (linear) HEAD after rebase, got parents: %q", parents)
	}

	// The branch is now strictly ahead, so a normal push fast-forwards origin.
	if err := git.Push(workDir); err != nil {
		t.Fatalf("Push after rebase: %v", err)
	}

	// The freshly-pushed origin contains both commits.
	verify := cloneRepo(t, originDir)
	for _, f := range []string{"remote.txt", "local.txt"} {
		if _, err := os.Stat(filepath.Join(verify, f)); err != nil {
			t.Errorf("expected %s on origin after push: %v", f, err)
		}
	}
}

func TestPullRebase_LeavesConflictInPlace(t *testing.T) {
	workDir, originDir := initRepoWithRemote(t)

	// A shared file exists on main and origin.
	makeCommit(t, workDir, "shared.txt", "base\n", "base content")
	mustRunGit(t, workDir, "push", "origin", "main")

	// Origin and local edit the same lines of shared.txt differently.
	advanceOriginMain(t, originDir, "shared.txt", "remote change\n")
	makeCommit(t, workDir, "shared.txt", "local change\n", "local edit")

	if _, err := git.PullRebase(workDir, "main"); err == nil {
		t.Fatal("expected PullRebase to fail on conflicting changes")
	}

	conflicts, err := git.UnmergedFiles(workDir)
	if err != nil {
		t.Fatalf("UnmergedFiles: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0] != "shared.txt" {
		t.Errorf("expected [shared.txt] unmerged, got %v", conflicts)
	}

	// The repo must be left in a rebasing state for manual resolution.
	ops, err := git.InProgressOperations(workDir)
	if err != nil {
		t.Fatalf("InProgressOperations: %v", err)
	}
	if !containsString(ops, "rebase") {
		t.Errorf("expected an in-progress rebase, got %v", ops)
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

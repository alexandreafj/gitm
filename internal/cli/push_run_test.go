package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func TestRunPush_PushesAheadBranch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// A feature branch with a local commit that origin does not have yet.
	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	writeFile(t, dir, "feature.go", "package main\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "feature work")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runPushWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
		t.Fatalf("runPushWithUI: %v", err)
	}

	// rev-parse on the bare origin resolves the branch ref only if it was pushed.
	mustRunGit(t, originDir, "rev-parse", "feature/x")
}

func TestRunPush_RecoversDivergedBranch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// Remote advances; local advances → the branch has diverged.
	advanceOriginMain(t, originDir, "remote.txt", "remote\n")
	writeFile(t, dir, "local.txt", "local\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "local work")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	output := captureOutput(t, func() {
		if err := runPushWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
			t.Fatalf("runPushWithUI: %v", err)
		}
	})

	if !strings.Contains(output, "rebased") {
		t.Errorf("expected output to report the rebase recovery, got:\n%s", output)
	}

	// Origin now contains both the remote commit and the rebased local commit.
	verify := cloneRepo(t, originDir)
	for _, f := range []string{"remote.txt", "local.txt"} {
		if _, statErr := os.Stat(filepath.Join(verify, f)); statErr != nil {
			t.Errorf("expected %s on origin after diverged push recovery: %v", f, statErr)
		}
	}
}

func TestRunPush_LeavesRebaseConflictInPlace(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// A shared file lives on main and origin.
	writeFile(t, dir, "shared.txt", "base\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "add shared")
	mustRunGit(t, dir, "push", "origin", "main")

	// Origin and local edit the same lines differently → rebase conflict.
	advanceOriginMain(t, originDir, "shared.txt", "remote change\n")
	writeFile(t, dir, "shared.txt", "local change\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "local edit")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// A rebase conflict is an expected outcome, not a hard failure → no error.
	output := captureOutput(t, func() {
		if err := runPushWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
			t.Fatalf("runPushWithUI returned error on conflict (should be nil): %v", err)
		}
	})

	if !strings.Contains(output, "rebase conflict") {
		t.Errorf("expected output to report the rebase conflict, got:\n%s", output)
	}

	// The repo must be left in a rebasing state for manual resolution.
	ops, err := git.InProgressOperations(dir)
	if err != nil {
		t.Fatalf("InProgressOperations: %v", err)
	}
	if !containsAlias(ops, "rebase") {
		t.Errorf("expected an in-progress rebase, got %v", ops)
	}
}

func TestRunPush_SkipsWhenNothingToPush(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	output := captureOutput(t, func() {
		if err := runPushWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
			t.Fatalf("runPushWithUI: %v", err)
		}
	})

	if !strings.Contains(output, "nothing to push") {
		t.Errorf("expected an up-to-date repo to be skipped, got:\n%s", output)
	}
}

func TestRunPush_RepoFlagBypassesTUI(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	writeFile(t, dir, "feature.go", "package main\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "feature work")

	if _, err := database.AddRepository("repo1", "repo1", dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// selectErr makes the test fail if MultiSelect is consulted — proving --repo
	// bypasses the interactive picker.
	ui := fakeUI{selectErr: fmt.Errorf("MultiSelect should not be called with --repo")}
	if err := runPushWithUI(ui, false, []string{"repo1"}); err != nil {
		t.Fatalf("runPushWithUI: %v", err)
	}

	mustRunGit(t, originDir, "rev-parse", "feature/x")
}

func TestRunPush_AllFlagBypassesTUI(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	writeFile(t, dir, "feature.go", "package main\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "feature work")

	if _, err := database.AddRepository("repo1", "repo1", dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	ui := fakeUI{selectErr: fmt.Errorf("MultiSelect should not be called with --all")}
	if err := runPushWithUI(ui, true, nil); err != nil {
		t.Fatalf("runPushWithUI: %v", err)
	}

	mustRunGit(t, originDir, "rev-parse", "feature/x")
}

func TestRunPush_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	if err := runPushWithUI(fakeUI{}, false, nil); err != nil {
		t.Fatalf("runPushWithUI with no repos: %v", err)
	}
}

func TestRunPush_ReturnsErrorOnFailure(t *testing.T) {
	database = setupTestDB(t)

	// A repo whose path does not exist makes git fail — a genuine error that must
	// surface (exit non-zero), unlike a rebase conflict which is a skip.
	repo, err := database.AddRepository("broken", "broken", "/nonexistent/path", "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err = runPushWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil)
	if err == nil {
		t.Fatal("expected error when a repo fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to push") {
		t.Errorf("error = %q, want to contain \"failed to push\"", err.Error())
	}
}

func containsAlias(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

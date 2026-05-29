package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestSyncCmdExists(t *testing.T) {
	if syncCmd() == nil {
		t.Fatal("syncCmd() returned nil")
	}
}

func TestSyncCmdHasUse(t *testing.T) {
	if cmd := syncCmd(); cmd.Use != "sync" {
		t.Errorf("syncCmd Use = %q, want %q", cmd.Use, "sync")
	}
}

func TestSyncCmdHasShort(t *testing.T) {
	if syncCmd().Short == "" {
		t.Error("syncCmd has empty Short description")
	}
}

func TestSyncCmdIsRunnable(t *testing.T) {
	if syncCmd().RunE == nil {
		t.Error("syncCmd has no RunE function")
	}
}

func TestSyncCmdHasRepoFlag(t *testing.T) {
	f := syncCmd().Flags().Lookup("repo")
	if f == nil {
		t.Fatal("syncCmd missing --repo flag")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

func TestSyncCmdHasAllFlag(t *testing.T) {
	f := syncCmd().Flags().Lookup("all")
	if f == nil {
		t.Fatal("syncCmd missing --all flag")
	}
	if f.Shorthand != "a" {
		t.Errorf("--all shorthand = %q, want %q", f.Shorthand, "a")
	}
}

// advanceOriginMain pushes a new commit to origin's main branch by cloning the
// bare origin, committing, and pushing — simulating master moving ahead.
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

func TestRunSync_MergesDefaultIntoCurrent(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr != nil {
		t.Errorf("expected frommain.go to be merged from origin/main: %v", statErr)
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("expected to stay on feature/x, got %q", head)
	}
}

func TestRunSync_SkipsDirty(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	// Uncommitted change to a tracked file → repo must be skipped.
	writeFile(t, dir, "README.md", "dirty change\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	// Nothing should have been merged into the dirty repo.
	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr == nil {
		t.Error("dirty repo should have been skipped, but origin/main was merged")
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("expected to stay on feature/x, got %q", head)
	}
}

func TestRunSync_SkipsWhenOnDefaultBranch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// Stay on the default branch (main); advance origin/main.
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	// On the default branch sync is a no-op (it does not even merge origin/main).
	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr == nil {
		t.Error("on default branch sync should be a no-op, but origin/main was merged")
	}
	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("expected to stay on main, got %q", head)
	}
}

func TestRunSync_LeavesConflictInPlace(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// Establish a shared file on main + origin.
	writeFile(t, dir, "shared.txt", "base\n")
	mustRunGit(t, dir, "add", "shared.txt")
	mustRunGit(t, dir, "commit", "-m", "add shared on main")
	mustRunGit(t, dir, "push", "origin", "main")

	// Feature branch edits the shared file one way…
	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	writeFile(t, dir, "shared.txt", "feature change\n")
	mustRunGit(t, dir, "add", "shared.txt")
	mustRunGit(t, dir, "commit", "-m", "feature edit")

	// …while origin/main edits the same lines another way.
	advanceOriginMain(t, originDir, "shared.txt", "main change\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// A conflict is an expected outcome, not a hard failure → no error returned.
	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil); err != nil {
		t.Fatalf("runSyncWithUI returned error on conflict (should be nil): %v", err)
	}

	// The repo must be left in a merging state for manual resolution.
	if _, statErr := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); statErr != nil {
		t.Errorf("expected MERGE_HEAD (conflict left in place): %v", statErr)
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("expected to stay on feature/x, got %q", head)
	}
}

func TestRunSync_RepoFlagBypassesTUI(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	if _, err := database.AddRepository("repo1", "repo1", dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// selectErr ensures the test fails if MultiSelect is consulted — proving
	// --repo bypasses the interactive picker.
	ui := fakeUI{selectErr: fmt.Errorf("MultiSelect should not be called with --repo")}
	if err := runSyncWithUI(ui, false, []string{"repo1"}); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr != nil {
		t.Errorf("expected frommain.go to be merged: %v", statErr)
	}
}

func TestRunSync_AllFlagBypassesTUI(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	if _, err := database.AddRepository("repo1", "repo1", dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	ui := fakeUI{selectErr: fmt.Errorf("MultiSelect should not be called with --all")}
	if err := runSyncWithUI(ui, true, nil); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr != nil {
		t.Errorf("expected frommain.go to be merged: %v", statErr)
	}
}

func TestRunSync_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	if err := runSyncWithUI(fakeUI{}, false, nil); err != nil {
		t.Fatalf("runSyncWithUI with no repos: %v", err)
	}
}

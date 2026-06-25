package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestSyncCmdExists(t *testing.T) {
	if syncCmd() == nil {
		t.Fatal("syncCmd() returned nil")
	}
}

func TestSyncCmdHasUse(t *testing.T) {
	if cmd := syncCmd(); cmd.Use != "sync [branch]" {
		t.Errorf("syncCmd Use = %q, want %q", cmd.Use, "sync [branch]")
	}
}

func TestSyncCmdAcceptsBranchArg(t *testing.T) {
	cmd := syncCmd()
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("syncCmd should accept zero args, got error: %v", err)
	}
	if err := cmd.Args(cmd, []string{"master-raw"}); err != nil {
		t.Errorf("syncCmd should accept one positional branch arg, got error: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("syncCmd should reject more than one positional arg")
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

// advanceOriginBranch creates (or fast-forwards) an arbitrary branch on origin by
// cloning the bare origin, branching off its default, committing, and pushing —
// used to set up a non-default branch (e.g. "staging") for sync to merge.
func advanceOriginBranch(t *testing.T, originDir, branch, filename, content string) {
	t.Helper()
	clone := cloneRepo(t, originDir)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	mustRunGit(t, clone, "config", "commit.gpgsign", "false")
	mustRunGit(t, clone, "checkout", "-B", branch)
	writeFile(t, clone, filename, content)
	mustRunGit(t, clone, "add", ".")
	mustRunGit(t, clone, "commit", "-m", "advance "+branch+": "+filename)
	mustRunGit(t, clone, "push", "origin", branch)
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

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, ""); err != nil {
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

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, ""); err != nil {
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

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, ""); err != nil {
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
	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, ""); err != nil {
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
	if err := runSyncWithUI(ui, false, []string{"repo1"}, ""); err != nil {
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
	if err := runSyncWithUI(ui, true, nil, ""); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr != nil {
		t.Errorf("expected frommain.go to be merged: %v", statErr)
	}
}

func TestRunSync_AllowsUntrackedFiles(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	// Untracked files should not block sync — they don't interfere with merges.
	writeFile(t, dir, "scratch.txt", "untracked\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, ""); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr != nil {
		t.Errorf("expected frommain.go to be merged, but sync was blocked: %v", statErr)
	}
}

func TestRunSync_ReturnsErrorOnFailure(t *testing.T) {
	database = setupTestDB(t)

	// A repo whose path does not exist makes git fail — a genuine error that
	// must surface (exit non-zero), unlike a merge conflict which is a skip.
	repo, err := database.AddRepository("broken", "broken", "/nonexistent/path", "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err = runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, "")
	if err == nil {
		t.Fatal("expected error when a repo fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to sync") {
		t.Errorf("error = %q, want to contain \"failed to sync\"", err.Error())
	}
}

func TestRunSync_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	if err := runSyncWithUI(fakeUI{}, false, nil, ""); err != nil {
		t.Fatalf("runSyncWithUI with no repos: %v", err)
	}
}

func TestRunSync_MergesSpecifiedBranch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// A non-default branch "staging" exists on origin with a unique file.
	advanceOriginBranch(t, originDir, "staging", "fromstaging.go", "package staging\n")

	mustRunGit(t, dir, "checkout", "-b", "feature/x")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// Sync the explicit branch instead of the default (main).
	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, "staging"); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "fromstaging.go")); statErr != nil {
		t.Errorf("expected fromstaging.go to be merged from origin/staging: %v", statErr)
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("expected to stay on feature/x, got %q", head)
	}
}

func TestRunSync_SkipsWhenOnSpecifiedBranch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	// Check out staging locally so we are *on* the branch being synced.
	advanceOriginBranch(t, originDir, "staging", "fromstaging.go", "package staging\n")
	mustRunGit(t, dir, "fetch", "origin")
	mustRunGit(t, dir, "checkout", "staging")

	// Advance origin/main with a unique file; if sync wrongly merged the default
	// branch, this file would appear in the working tree.
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, "staging"); err != nil {
		t.Fatalf("runSyncWithUI: %v", err)
	}

	// Syncing a branch into itself is a no-op: nothing from the default branch
	// should have been merged.
	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr == nil {
		t.Error("on the specified branch sync should be a no-op, but origin/main was merged")
	}
	if head := gitCurrentBranch(t, dir); head != "staging" {
		t.Fatalf("expected to stay on staging, got %q", head)
	}
}

func TestRunSync_SpecifiedBranchNotFound(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)

	mustRunGit(t, dir, "checkout", "-b", "feature/x")
	// Advance origin/main so the default-branch path would merge something — proving
	// the requested (missing) branch, not the default, drives the outcome.
	advanceOriginMain(t, originDir, "frommain.go", "package main\n")

	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// A branch that exists neither locally nor on origin is a skip, not an error.
	if err := runSyncWithUI(fakeUI{selectRepos: []*db.Repository{repo}}, false, nil, "nope-not-real"); err != nil {
		t.Fatalf("runSyncWithUI returned error for missing branch (should skip): %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "frommain.go")); statErr == nil {
		t.Error("missing branch should be skipped, but the default branch was merged instead")
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("expected to stay on feature/x, got %q", head)
	}
}

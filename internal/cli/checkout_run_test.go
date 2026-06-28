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

func TestRunCheckoutWithUI_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	ui := fakeUI{}

	if err := runCheckoutWithUI(ui, []string{""}, nil); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}
}

func TestRunCheckoutDefault(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
		t.Fatalf("runCheckoutDefault: %v", err)
	}

	head := gitCurrentBranch(t, dir)
	if head == "" {
		t.Error("expected branch to be set")
	}
	if head != "main" {
		t.Fatalf("head = %q, want %q", head, "main")
	}
}

func TestRunCheckoutBranch_SkipsDirty(t *testing.T) {
	database = setupTestDB(t)
	repo, dir := newRepo(t, database, "repo1")

	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := runCheckoutBranch([]*db.Repository{repo}, "feature/test"); err != nil {
		t.Fatalf("runCheckoutBranch: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("head = %q, want %q", head, "main")
	}
}

func TestRunCheckoutBranch_NotFound(t *testing.T) {
	database = setupTestDB(t)
	repo, _ := newRepo(t, database, "repo1")

	if err := runCheckoutBranch([]*db.Repository{repo}, "missing-branch"); err != nil {
		t.Fatalf("runCheckoutBranch: %v", err)
	}
}

func TestRunCheckoutBranch_Checkout(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	mustRunGit(t, dir, "checkout", "-b", "feature/test")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "feature/test")
	mustRunGit(t, dir, "checkout", "main")

	if err := runCheckoutBranch([]*db.Repository{repo}, "feature/test"); err != nil {
		t.Fatalf("runCheckoutBranch: %v", err)
	}
}

func TestRunCheckoutInteractive(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	mustRunGit(t, dir, "checkout", "-b", "feature/test")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "feature/test")
	mustRunGit(t, dir, "checkout", "main")

	ui := fakeUI{branchName: "feature/test"}
	if err := runCheckoutInteractive([]*db.Repository{repo}, ui); err != nil {
		t.Fatalf("runCheckoutInteractive: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "feature/test" {
		t.Fatalf("head = %q, want %q", head, "feature/test")
	}
}

func TestRunCheckoutWithUI_RepoFlag_DefaultBranch_SingleRepo(t *testing.T) {
	database = setupTestDB(t)

	dir1, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", dir1, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	dir2, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", dir2, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	// Put both repos on a feature branch.
	mustRunGit(t, dir1, "checkout", "-b", "feature/work")
	mustRunGit(t, dir2, "checkout", "-b", "feature/work")

	// Checkout default branch only for repo1.
	if err := runCheckoutWithUI(fakeUI{}, []string{"master"}, []string{"repo1"}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}

	if head := gitCurrentBranch(t, dir1); head != "main" {
		t.Fatalf("repo1 head = %q, want main", head)
	}
	if head := gitCurrentBranch(t, dir2); head != "feature/work" {
		t.Fatalf("repo2 should stay on feature/work, got %q", head)
	}
}

func TestRunCheckoutWithUI_RepoFlag_DefaultBranch_MultipleRepos(t *testing.T) {
	database = setupTestDB(t)

	dir1, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", dir1, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	dir2, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", dir2, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	dir3, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo3", "repo3", dir3, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	mustRunGit(t, dir1, "checkout", "-b", "feature/work")
	mustRunGit(t, dir2, "checkout", "-b", "feature/work")
	mustRunGit(t, dir3, "checkout", "-b", "feature/work")

	if err := runCheckoutWithUI(fakeUI{}, []string{"master"}, []string{"repo1", "repo3"}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}

	if head := gitCurrentBranch(t, dir1); head != "main" {
		t.Fatalf("repo1 head = %q, want main", head)
	}
	if head := gitCurrentBranch(t, dir2); head != "feature/work" {
		t.Fatalf("repo2 should stay on feature/work, got %q", head)
	}
	if head := gitCurrentBranch(t, dir3); head != "main" {
		t.Fatalf("repo3 head = %q, want main", head)
	}
}

func TestRunCheckoutWithUI_RepoFlag_SpecificBranch(t *testing.T) {
	database = setupTestDB(t)

	dir1, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", dir1, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	dir2, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", dir2, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	mustRunGit(t, dir1, "checkout", "-b", "feature/targeted")
	mustRunGit(t, dir1, "push", "--set-upstream", "origin", "feature/targeted")
	mustRunGit(t, dir1, "checkout", "main")

	mustRunGit(t, dir2, "checkout", "-b", "feature/targeted")
	mustRunGit(t, dir2, "push", "--set-upstream", "origin", "feature/targeted")
	mustRunGit(t, dir2, "checkout", "main")

	if err := runCheckoutWithUI(fakeUI{}, []string{"feature/targeted"}, []string{"repo1"}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}

	if head := gitCurrentBranch(t, dir1); head != "feature/targeted" {
		t.Fatalf("repo1 head = %q, want feature/targeted", head)
	}
	if head := gitCurrentBranch(t, dir2); head != "main" {
		t.Fatalf("repo2 should stay on main, got %q", head)
	}
}

func TestRunCheckoutWithUI_RepoFlag_UnknownAlias(t *testing.T) {
	database = setupTestDB(t)

	dir1, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", dir1, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	err := runCheckoutWithUI(fakeUI{}, []string{"master"}, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain \"not found\"", err.Error())
	}
}

func TestRunCheckoutWithUI_RepoFlag_EmptySlice(t *testing.T) {
	database = setupTestDB(t)

	dir1, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", dir1, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	mustRunGit(t, dir1, "checkout", "-b", "feature/work")

	// Empty slice should behave like nil — update all repos.
	if err := runCheckoutWithUI(fakeUI{}, []string{"master"}, []string{}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}

	if head := gitCurrentBranch(t, dir1); head != "main" {
		t.Fatalf("repo1 head = %q, want main", head)
	}
}

func TestRunCheckoutBranch_RemoteOnly(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	clone2Dir := cloneRepo(t, originDir)
	mustRunGit(t, clone2Dir, "config", "user.email", "test@example.com")
	mustRunGit(t, clone2Dir, "config", "user.name", "Test User")
	mustRunGit(t, clone2Dir, "checkout", "-b", "feature/remote-only")
	writeFile(t, clone2Dir, "remote.txt", "from remote\n")
	mustRunGit(t, clone2Dir, "add", ".")
	mustRunGit(t, clone2Dir, "commit", "-m", "remote commit")
	mustRunGit(t, clone2Dir, "push", "--set-upstream", "origin", "feature/remote-only")

	// The branch only exists on origin, not locally in our working repo.
	if err := runCheckoutBranch([]*db.Repository{repo}, "feature/remote-only"); err != nil {
		t.Fatalf("runCheckoutBranch: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "feature/remote-only" {
		t.Fatalf("head = %q, want %q", head, "feature/remote-only")
	}
}

func TestRunCheckoutBranch_DryRunRemoteOnlyDoesNotFetchOrSwitch(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	clone2Dir := cloneRepo(t, originDir)
	mustRunGit(t, clone2Dir, "config", "user.email", "test@example.com")
	mustRunGit(t, clone2Dir, "config", "user.name", "Test User")
	mustRunGit(t, clone2Dir, "checkout", "-b", "feature/remote-only")
	writeFile(t, clone2Dir, "remote.txt", "from remote\n")
	mustRunGit(t, clone2Dir, "add", ".")
	mustRunGit(t, clone2Dir, "commit", "-m", "remote commit")
	mustRunGit(t, clone2Dir, "push", "--set-upstream", "origin", "feature/remote-only")

	output := captureOutput(t, func() {
		if err := runCheckoutBranchDryRun([]*db.Repository{repo}, "feature/remote-only", true); err != nil {
			t.Fatalf("runCheckoutBranchDryRun: %v", err)
		}
	})

	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("head = %q, want main after dry-run", head)
	}
	if git.BranchExists(dir, "feature/remote-only") {
		t.Fatal("dry-run should not fetch/create the remote-only branch locally")
	}
	if !strings.Contains(output, "git fetch origin -- feature/remote-only") {
		t.Fatalf("expected fetch preview, got:\n%s", output)
	}
	if !strings.Contains(output, "git checkout feature/remote-only") {
		t.Fatalf("expected checkout preview, got:\n%s", output)
	}
}

func TestRunCheckoutDefault_ReturnsErrorOnFailure(t *testing.T) {
	database = setupTestDB(t)

	repo := &db.Repository{
		ID:            1,
		Alias:         "broken",
		Path:          "/nonexistent/path",
		DefaultBranch: "main",
	}

	err := runCheckoutDefault([]*db.Repository{repo})
	if err == nil {
		t.Fatal("expected error when repo fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to checkout") {
		t.Errorf("error = %q, want to contain \"failed to checkout\"", err.Error())
	}
}

func TestRunCheckoutDefault_NonConflictingDirtyFile(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)

	writeFile(t, dir, "config.txt", "original\n")
	mustRunGit(t, dir, "add", "config.txt")
	mustRunGit(t, dir, "commit", "-m", "add config")
	mustRunGit(t, dir, "push")

	mustRunGit(t, dir, "checkout", "-b", "feature")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "feature")
	mustRunGit(t, dir, "commit", "--allow-empty", "-m", "feature work")

	writeFile(t, dir, "config.txt", "local modification\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
		t.Fatalf("should succeed with non-conflicting dirty file: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("head = %q, want main", head)
	}
}

func TestRunCheckoutDefault_ConflictingDirtyFile(t *testing.T) {
	database = setupTestDB(t)
	dir := initRepo(t)

	writeFile(t, dir, "conflict.txt", "main-version\n")
	mustRunGit(t, dir, "add", "conflict.txt")
	mustRunGit(t, dir, "commit", "-m", "add conflict on main")

	mustRunGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "conflict.txt", "feature-version\n")
	mustRunGit(t, dir, "add", "conflict.txt")
	mustRunGit(t, dir, "commit", "-m", "change conflict on feature")

	writeFile(t, dir, "conflict.txt", "local dirty\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
		t.Fatalf("should skip (not error) for conflicting dirty file: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "feature" {
		t.Fatalf("should stay on feature (skipped), got %s", head)
	}
}

func TestCheckoutBranchInRepo_NonConflictingDirtyFile(t *testing.T) {
	dir, _, _ := initRepoWithRemote(t)

	writeFile(t, dir, "safe.txt", "original\n")
	mustRunGit(t, dir, "add", "safe.txt")
	mustRunGit(t, dir, "commit", "-m", "add safe file")
	mustRunGit(t, dir, "push")

	mustRunGit(t, dir, "checkout", "-b", "target")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "target")
	mustRunGit(t, dir, "commit", "--allow-empty", "-m", "target commit")
	mustRunGit(t, dir, "checkout", "main")

	writeFile(t, dir, "safe.txt", "dirty but safe\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	msg, skipReason, err := checkoutBranchInRepo(repo, "target")
	if err != nil {
		t.Fatalf("should succeed: %v", err)
	}
	if skipReason != "" {
		t.Fatalf("should not skip, got: %s", skipReason)
	}
	if msg == "" {
		t.Fatal("expected success message")
	}

	if head := gitCurrentBranch(t, dir); head != "target" {
		t.Fatalf("head = %q, want target", head)
	}
}

func TestCheckoutBranchInRepo_ConflictingDirtyFile(t *testing.T) {
	dir := initRepo(t)

	writeFile(t, dir, "conflict.txt", "main-version\n")
	mustRunGit(t, dir, "add", "conflict.txt")
	mustRunGit(t, dir, "commit", "-m", "add conflict on main")

	mustRunGit(t, dir, "checkout", "-b", "target")
	writeFile(t, dir, "conflict.txt", "target-version\n")
	mustRunGit(t, dir, "add", "conflict.txt")
	mustRunGit(t, dir, "commit", "-m", "change conflict on target")

	mustRunGit(t, dir, "checkout", "main")
	writeFile(t, dir, "conflict.txt", "local dirty\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	_, skipReason, err := checkoutBranchInRepo(repo, "target")
	if err != nil {
		t.Fatalf("should skip, not error: %v", err)
	}
	if skipReason == "" {
		t.Fatal("expected skip reason for conflicting dirty file")
	}
	if !strings.Contains(skipReason, "conflict") {
		t.Errorf("skip reason = %q, want to contain 'conflict'", skipReason)
	}

	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("should stay on main (skipped), got %s", head)
	}
}

func TestCheckoutInteractive_NonConflictingDirtyFile(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)

	writeFile(t, dir, "tracked.txt", "original\n")
	mustRunGit(t, dir, "add", "tracked.txt")
	mustRunGit(t, dir, "commit", "-m", "add tracked file")
	mustRunGit(t, dir, "push")

	mustRunGit(t, dir, "checkout", "-b", "target")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "target")
	mustRunGit(t, dir, "commit", "--allow-empty", "-m", "target commit")
	mustRunGit(t, dir, "checkout", "main")

	writeFile(t, dir, "tracked.txt", "dirty but non-conflicting\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	ui := fakeUI{selectRepos: []*db.Repository{repo}, branchName: "target"}

	if err := runCheckoutInteractive([]*db.Repository{repo}, ui); err != nil {
		t.Fatalf("interactive checkout should succeed with non-conflicting dirty: %v", err)
	}

	if head := gitCurrentBranch(t, dir); head != "target" {
		t.Fatalf("head = %q, want target", head)
	}
}

func TestCheckoutDefault_MultipleReposMixedDirtyState(t *testing.T) {
	database = setupTestDB(t)

	cleanDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, cleanDir, "checkout", "-b", "feature")
	mustRunGit(t, cleanDir, "push", "--set-upstream", "origin", "feature")
	mustRunGit(t, cleanDir, "commit", "--allow-empty", "-m", "feature")

	conflictDir, _, _ := initRepoWithRemote(t)
	writeFile(t, conflictDir, "conflict.txt", "v1\n")
	mustRunGit(t, conflictDir, "add", "conflict.txt")
	mustRunGit(t, conflictDir, "commit", "-m", "add conflict")
	mustRunGit(t, conflictDir, "push")
	mustRunGit(t, conflictDir, "checkout", "-b", "feature")
	mustRunGit(t, conflictDir, "push", "--set-upstream", "origin", "feature")
	writeFile(t, conflictDir, "conflict.txt", "v2\n")
	mustRunGit(t, conflictDir, "add", "conflict.txt")
	mustRunGit(t, conflictDir, "commit", "-m", "change conflict")
	writeFile(t, conflictDir, "conflict.txt", "local dirty\n")

	nonConflictDir, _, _ := initRepoWithRemote(t)
	writeFile(t, nonConflictDir, "safe.txt", "original\n")
	mustRunGit(t, nonConflictDir, "add", "safe.txt")
	mustRunGit(t, nonConflictDir, "commit", "-m", "add safe")
	mustRunGit(t, nonConflictDir, "push")
	mustRunGit(t, nonConflictDir, "checkout", "-b", "feature")
	mustRunGit(t, nonConflictDir, "push", "--set-upstream", "origin", "feature")
	mustRunGit(t, nonConflictDir, "commit", "--allow-empty", "-m", "feature")
	writeFile(t, nonConflictDir, "safe.txt", "dirty but safe\n")

	repos := []*db.Repository{
		{ID: 1, Alias: "clean", Path: cleanDir, DefaultBranch: "main"},
		{ID: 2, Alias: "conflict", Path: conflictDir, DefaultBranch: "main"},
		{ID: 3, Alias: "non-conflict", Path: nonConflictDir, DefaultBranch: "main"},
	}

	if err := runCheckoutDefault(repos); err != nil {
		t.Fatalf("runCheckoutDefault failed: %v", err)
	}

	if b := gitCurrentBranch(t, cleanDir); b != "main" {
		t.Errorf("clean repo: expected main, got %s", b)
	}
	if b := gitCurrentBranch(t, conflictDir); b != "feature" {
		t.Errorf("conflict repo: expected to stay on feature, got %s", b)
	}
	if b := gitCurrentBranch(t, nonConflictDir); b != "main" {
		t.Errorf("non-conflict repo: expected main (dirty carried forward), got %s", b)
	}
}

func TestIsCheckoutConflict(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"error: Your local changes to the following files would be overwritten by checkout", true},
		{"error: Your local changes to the following files would be overwritten by merge", true},
		{"error: pathspec 'nonexistent' did not match any file(s) known to git", false},
		{"exit status 1", false},
	}
	for _, tt := range tests {
		got := isCheckoutConflict(fmt.Errorf("%s", tt.msg))
		if got != tt.want {
			t.Errorf("isCheckoutConflict(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestCheckoutConflictSkip_NonConflictError(t *testing.T) {
	skip, _ := checkoutConflictSkip("/nonexistent", fmt.Errorf("pathspec not found"))
	if skip {
		t.Error("should not skip for non-conflict errors")
	}
}

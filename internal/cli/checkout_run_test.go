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
	noOrigin, _ := newRepo(t, database, "no-origin")
	dir, _, _ := initRepoWithRemote(t)
	withOrigin, err := database.AddRepository("with-origin", "with-origin", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	repos := []*db.Repository{noOrigin, withOrigin}

	want := `branch "missing-branch" not found (local or remote)`
	for _, repo := range repos {
		_, skipReason, err := checkoutBranchInRepo(repo, "missing-branch")
		if err != nil {
			t.Fatalf("%s: checkoutBranchInRepo: %v", repo.Alias, err)
		}
		if skipReason != want {
			t.Fatalf("%s: skip reason = %q, want %q", repo.Alias, skipReason, want)
		}
	}

	if err := runCheckoutBranch(repos, "missing-branch"); err != nil {
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

	output := captureOutput(t, func() {
		if err := runCheckoutBranch([]*db.Repository{repo}, "feature/remote-only"); err != nil {
			t.Fatalf("runCheckoutBranch: %v", err)
		}
	})

	if head := gitCurrentBranch(t, dir); head != "feature/remote-only" {
		t.Fatalf("head = %q, want %q", head, "feature/remote-only")
	}
	if !strings.Contains(output, "on feature/remote-only — fetched from origin") {
		t.Fatalf("expected fetched-from-origin message, got:\n%s", output)
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
	if strings.Contains(output, "git pull") {
		t.Fatalf("a fetched branch is not pulled, so the preview should not list a pull, got:\n%s", output)
	}
}

func TestCheckoutBranchDryRun_DirtyCurrentBranchShowsSkip(t *testing.T) {
	dir := initRepo(t)
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	writeFile(t, dir, "README.md", "uncommitted\n")
	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}

	items := checkoutBranchDryRunItems([]*db.Repository{repo}, "AA-19432")
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	want := "already on AA-19432 with uncommitted changes — pull skipped"
	if items[0].skipReason != want {
		t.Fatalf("skip reason = %q, want %q", items[0].skipReason, want)
	}
	if len(items[0].actions) != 0 {
		t.Fatalf("actions = %v, want none for known skip", items[0].actions)
	}
}

func TestCheckoutDefaultDryRun_DirtyCurrentBranchShowsSkip(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "uncommitted\n")
	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}

	items := checkoutDefaultDryRunItems([]*db.Repository{repo})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	want := "already on main with uncommitted changes — pull skipped"
	if items[0].skipReason != want {
		t.Fatalf("skip reason = %q, want %q", items[0].skipReason, want)
	}
	if len(items[0].actions) != 0 {
		t.Fatalf("actions = %v, want none for known skip", items[0].actions)
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

func TestCheckoutDefault_DirtyCurrentBranchSkipsPull(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	writeFile(t, dir, "work.txt", "committed\n")
	mustRunGit(t, dir, "add", "work.txt")
	mustRunGit(t, dir, "commit", "-m", "add work file")
	mustRunGit(t, dir, "push")
	mustRunGit(t, dir, "config", "pull.rebase", "true")
	writeFile(t, dir, "work.txt", "uncommitted\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
			t.Fatalf("runCheckoutDefault: %v", err)
		}
	})
	if !strings.Contains(output, "SKIPPED: already on main with uncommitted changes — pull skipped") {
		t.Fatalf("output does not contain dirty current-branch skip:\n%s", output)
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

func TestCheckoutBranchInRepo_DirtyCurrentBranchSkipsPull(t *testing.T) {
	dir, _, _ := initRepoWithRemote(t)

	writeFile(t, dir, "work.txt", "committed\n")
	mustRunGit(t, dir, "add", "work.txt")
	mustRunGit(t, dir, "commit", "-m", "add work file")
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "AA-19432")
	mustRunGit(t, dir, "config", "pull.rebase", "true")
	writeFile(t, dir, "work.txt", "uncommitted\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "AA-19432")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if message != "" {
		t.Fatalf("message = %q, want empty", message)
	}
	want := "already on AA-19432 with uncommitted changes — pull skipped"
	if skipReason != want {
		t.Fatalf("skip reason = %q, want %q", skipReason, want)
	}
	content, err := os.ReadFile(filepath.Join(dir, "work.txt"))
	if err != nil {
		t.Fatalf("read work.txt: %v", err)
	}
	if got := string(content); got != "uncommitted\n" {
		t.Fatalf("work.txt = %q, want uncommitted work preserved", got)
	}
}

func TestCheckoutBranchInRepo_CleanCurrentBranchPullsRemoteChanges(t *testing.T) {
	dir, origin, _ := initRepoWithRemote(t)
	mustRunGit(t, dir, "checkout", "-b", "AA-19432")
	mustRunGit(t, dir, "push", "--set-upstream", "origin", "AA-19432")

	other := filepath.Join(t.TempDir(), "other")
	mustRunGit(t, t.TempDir(), "clone", origin, other)
	mustRunGit(t, other, "config", "user.email", "test@example.com")
	mustRunGit(t, other, "config", "user.name", "Test User")
	mustRunGit(t, other, "checkout", "AA-19432")
	writeFile(t, other, "remote.txt", "from GitHub\n")
	mustRunGit(t, other, "add", "remote.txt")
	mustRunGit(t, other, "commit", "-m", "advance remote branch")
	mustRunGit(t, other, "push")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "AA-19432")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if skipReason != "" {
		t.Fatalf("skip reason = %q, want empty", skipReason)
	}
	if message == "" {
		t.Fatal("expected pull result message")
	}
	content, err := os.ReadFile(filepath.Join(dir, "remote.txt"))
	if err != nil {
		t.Fatalf("read remote.txt: %v", err)
	}
	if got := string(content); got != "from GitHub\n" {
		t.Fatalf("remote.txt = %q, want pulled remote content", got)
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

func TestCheckoutBranch_NoUpstreamPullSkipped(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	// A branch that exists only locally — created outside gitm, never pushed.
	mustRunGit(t, repoDir, "branch", "feature/local-only")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runCheckoutWithUI(fakeUI{}, []string{"feature/local-only"}, nil); err != nil {
		t.Fatalf("checkout of a local-only branch should not fail on the pull: %v", err)
	}

	if branch := gitCurrentBranch(t, repoDir); branch != "feature/local-only" {
		t.Errorf("expected to be on feature/local-only, got %q", branch)
	}
}

func TestCheckoutDefault_NoRemotePullSkipped(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	mustRunGit(t, repoDir, "checkout", "-b", "feature/elsewhere")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runCheckoutWithUI(fakeUI{}, []string{"master"}, nil); err != nil {
		t.Fatalf("default checkout in a repo without a remote should not fail on the pull: %v", err)
	}

	if branch := gitCurrentBranch(t, repoDir); branch != "main" {
		t.Errorf("expected to be on main, got %q", branch)
	}
}

func TestCheckoutDefaultRefreshesStaleDefaultBranch(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "repo1", "main", "master")
	mustRunGit(t, repo.Path, "checkout", "main")

	if err := runCheckoutWithUI(fakeUI{}, []string{"main"}, []string{"repo1"}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}
	if branch := gitCurrentBranch(t, repo.Path); branch != "master" {
		t.Fatalf("current branch = %q, want master", branch)
	}
	stored, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master", stored.DefaultBranch)
	}
}

func TestCheckoutDefaultDryRunUsesLiveDefaultWithoutPersistence(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "repo1", "main", "master")
	mustRunGit(t, repo.Path, "checkout", "main")

	output := captureOutput(t, func() {
		if err := runCheckoutWithUIDryRun(fakeUI{}, []string{"master"}, []string{"repo1"}, true); err != nil {
			t.Fatalf("runCheckoutWithUIDryRun: %v", err)
		}
	})
	if !strings.Contains(output, "git checkout master") {
		t.Fatalf("dry-run output does not use live default branch:\n%s", output)
	}
	if branch := gitCurrentBranch(t, repo.Path); branch != "main" {
		t.Fatalf("dry-run changed current branch to %q", branch)
	}
	stored, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("dry-run stored default branch = %q, want stale main", stored.DefaultBranch)
	}
}

func TestCheckoutExplicitBranchSkipsDefaultRefresh(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "repo1", "main", "master")
	mustRunGit(t, repo.Path, "branch", "feature/explicit")
	mustRunGit(t, repo.Path, "checkout", "main")

	if err := runCheckoutWithUI(fakeUI{}, []string{"feature/explicit"}, []string{"repo1"}); err != nil {
		t.Fatalf("runCheckoutWithUI: %v", err)
	}
	stored, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("explicit checkout refreshed default branch to %q", stored.DefaultBranch)
	}
}

func TestRunCheckoutDefault_RepairsRenamedRemoteDefault(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRenamedRemoteDefault(t, "renamed")
	dir := repo.Path

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
			t.Errorf("runCheckoutDefault: %v", err)
		}
	})

	if !strings.Contains(output, "on master — default branch renamed from main; ") {
		t.Fatalf("output = %q, want the rename called out", output)
	}
	if branch := gitCurrentBranch(t, dir); branch != "master" {
		t.Fatalf("current branch = %q, want master", branch)
	}
	stored, err := database.GetRepository("renamed")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master", stored.DefaultBranch)
	}
	local, err := git.LocalDefaultBranch(dir)
	if err != nil {
		t.Fatalf("LocalDefaultBranch: %v", err)
	}
	if local != "master" {
		t.Fatalf("local origin/HEAD = %q, want master", local)
	}
}

func TestRunCheckoutDefault_RepairsWhenCheckoutFails(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)
	// A second remote that also has main makes "git checkout main" ambiguous
	// once the local branch is gone, so the checkout fails before any pull
	// could fetch origin/master.
	mustRunGit(t, dir, "remote", "add", "upstream", originDir)
	mustRunGit(t, dir, "fetch", "upstream")
	repo, err := database.AddRepository("renamed", "renamed", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, dir, "remote", "set-head", "origin", "main")
	mustRunGit(t, originDir, "branch", "-m", "main", "master")
	mustRunGit(t, originDir, "symbolic-ref", "HEAD", "refs/heads/master")
	mustRunGit(t, dir, "checkout", "-b", "feature/elsewhere")
	mustRunGit(t, dir, "branch", "-D", "main")

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
			t.Errorf("runCheckoutDefault: %v", err)
		}
	})

	if !strings.Contains(output, "renamed from main") {
		t.Fatalf("output = %q, want the rename called out", output)
	}
	if branch := gitCurrentBranch(t, dir); branch != "master" {
		t.Fatalf("current branch = %q, want master", branch)
	}
	stored, err := database.GetRepository("renamed")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master", stored.DefaultBranch)
	}
	if got := mustRunGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/master" {
		t.Fatalf("origin/HEAD = %q, want refs/remotes/origin/master", got)
	}
}

func TestRunCheckoutDefault_PullOverwriteFailureIsNotRetried(t *testing.T) {
	database = setupTestDB(t)
	dir, originDir, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("repo1", "repo1", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, dir, "remote", "set-head", "origin", "main")

	other := filepath.Join(t.TempDir(), "other")
	mustRunGit(t, t.TempDir(), "clone", originDir, other)
	mustRunGit(t, other, "config", "user.email", "test@example.com")
	mustRunGit(t, other, "config", "user.name", "Test User")
	writeFile(t, other, "README.md", "changed upstream\n")
	mustRunGit(t, other, "commit", "-am", "advance main")
	mustRunGit(t, other, "push")

	// The edit rides along on checkout, then blocks the fast-forward pull.
	mustRunGit(t, dir, "checkout", "-b", "feature/edit")
	writeFile(t, dir, "README.md", "local edit\n")
	trace := filepath.Join(t.TempDir(), "git-trace")
	t.Setenv("GIT_TRACE", trace)

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err == nil {
			t.Error("expected the blocked pull to be reported as a failure")
		}
	})

	if !strings.Contains(output, "would be overwritten") {
		t.Fatalf("output = %q, want the original pull error", output)
	}
	traced, err := os.ReadFile(trace)
	if err != nil {
		t.Fatalf("read git trace: %v", err)
	}
	if strings.Contains(string(traced), "ls-remote") {
		t.Fatal("a pull blocked by local changes should not ask origin for its default branch")
	}
}

func TestRunCheckoutDefault_UnreachableOriginKeepsPullError(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("offline", "offline", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, dir, "remote", "set-head", "origin", "main")
	mustRunGit(t, dir, "remote", "set-url", "origin", "/nonexistent")

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err == nil {
			t.Error("expected the failed pull to be reported")
		}
	})

	if !strings.Contains(output, "pull:") || strings.Contains(output, "fetch default branch") {
		t.Fatalf("output = %q, want only the original pull error", output)
	}
	stored, err := database.GetRepository("offline")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("stored default branch = %q, want main untouched", stored.DefaultBranch)
	}
	if got := mustRunGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/main" {
		t.Fatalf("origin/HEAD = %q, want refs/remotes/origin/main untouched", got)
	}
}

func TestRunCheckoutDefault_FetchesRecordedButUnfetchedDefault(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRenamedRemoteDefault(t, "renamed")
	// What doctor leaves behind: cache and origin/HEAD name master, but
	// origin/master was never fetched.
	if err := database.UpdateDefaultBranch("renamed", "master"); err != nil {
		t.Fatalf("UpdateDefaultBranch: %v", err)
	}
	repo.DefaultBranch = "master"
	mustRunGit(t, repo.Path, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/master")

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err != nil {
			t.Errorf("runCheckoutDefault: %v", err)
		}
	})

	if branch := gitCurrentBranch(t, repo.Path); branch != "master" {
		t.Fatalf("current branch = %q, want master", branch)
	}
	if !strings.Contains(output, "on master — ") || strings.Contains(output, "renamed from") {
		t.Fatalf("output = %q, want a plain success on master", output)
	}
}

func TestRunCheckoutDefault_RepairReportsPersistenceFailure(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRenamedRemoteDefault(t, "renamed")
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err == nil {
			t.Error("expected the cache write failure to be reported")
		}
	})

	if !strings.Contains(output, "persist default branch master for renamed") || !strings.Contains(output, "pull:") {
		t.Fatalf("output = %q, want the original pull failure plus persistence context", output)
	}
	if repo.DefaultBranch != "main" {
		t.Fatalf("in-memory default branch = %q, want main kept when the cache write fails", repo.DefaultBranch)
	}
}

// addRepoWithRenamedRemoteDefault registers a repo whose local origin/HEAD still
// names main after origin renamed its default branch to master.
func addRepoWithRenamedRemoteDefault(t *testing.T, alias string) *db.Repository {
	t.Helper()
	dir, originDir, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository(alias, alias, dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, dir, "remote", "set-head", "origin", "main")
	mustRunGit(t, originDir, "branch", "-m", "main", "master")
	mustRunGit(t, originDir, "symbolic-ref", "HEAD", "refs/heads/master")
	return repo
}

func TestRunCheckoutDefault_UnchangedRemoteDefaultKeepsOriginalError(t *testing.T) {
	database = setupTestDB(t)
	dir, _, _ := initRepoWithRemote(t)
	repo, err := database.AddRepository("diverged", "diverged", dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, dir, "remote", "set-head", "origin", "main")
	mustRunGit(t, dir, "config", "branch.main.merge", "refs/heads/vanished")

	output := captureOutput(t, func() {
		if err := runCheckoutDefault([]*db.Repository{repo}); err == nil {
			t.Error("expected the pull failure to be reported")
		}
	})

	if !strings.Contains(output, "pull:") || !strings.Contains(output, "vanished") {
		t.Fatalf("output = %q, want the original pull error", output)
	}
	stored, err := database.GetRepository("diverged")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("stored default branch = %q, want main kept while origin still advertises it", stored.DefaultBranch)
	}
}

func TestCheckoutBranchInRepo_RemoteOnlySkipsLsRemoteAndPull(t *testing.T) {
	dir, origin, _ := initRepoWithRemote(t)
	other := cloneRepo(t, origin)
	mustRunGit(t, other, "config", "user.email", "test@example.com")
	mustRunGit(t, other, "config", "user.name", "Test User")
	mustRunGit(t, other, "checkout", "-b", "feature/x")
	writeFile(t, other, "remote.txt", "from origin\n")
	mustRunGit(t, other, "add", "remote.txt")
	mustRunGit(t, other, "commit", "-m", "remote commit")
	mustRunGit(t, other, "push", "--set-upstream", "origin", "feature/x")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "feature/x")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if skipReason != "" {
		t.Fatalf("skip reason = %q, want empty", skipReason)
	}
	if !strings.Contains(message, "fetched from origin") {
		t.Fatalf("message = %q, want it to mention fetched from origin", message)
	}
	if head := gitCurrentBranch(t, dir); head != "feature/x" {
		t.Fatalf("head = %q, want feature/x", head)
	}
	content, err := os.ReadFile(filepath.Join(dir, "remote.txt"))
	if err != nil {
		t.Fatalf("read remote.txt: %v", err)
	}
	if got := string(content); got != "from origin\n" {
		t.Fatalf("remote.txt = %q, want the fetched tip's content", got)
	}
}

func TestCheckoutBranchInRepo_TagOnlyOnOriginIsSkippedAsNotFound(t *testing.T) {
	dir, origin, _ := initRepoWithRemote(t)
	other := cloneRepo(t, origin)
	mustRunGit(t, other, "tag", "v1")
	mustRunGit(t, other, "push", "origin", "v1")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	_, skipReason, err := checkoutBranchInRepo(repo, "v1")
	if err != nil {
		t.Fatalf("checkoutBranchInRepo: %v", err)
	}
	if want := `branch "v1" not found (local or remote)`; skipReason != want {
		t.Fatalf("skip reason = %q, want %q", skipReason, want)
	}
	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("head = %q, want main", head)
	}
}

func TestCheckoutBranchInRepo_GlobNameIsErrorAndKeepsChanges(t *testing.T) {
	dir, _, _ := initRepoWithRemote(t)
	writeFile(t, dir, "feature.txt", "committed\n")
	mustRunGit(t, dir, "add", "feature.txt")
	mustRunGit(t, dir, "commit", "-m", "add feature.txt")
	writeFile(t, dir, "feature.txt", "uncommitted\n")

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	message, skipReason, err := checkoutBranchInRepo(repo, "feat*")
	if err == nil {
		t.Fatalf("expected an error for a glob branch name, got message %q, skip reason %q", message, skipReason)
	}
	if message != "" || skipReason != "" {
		t.Fatalf("message = %q, skip reason = %q, want both empty alongside the error", message, skipReason)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, "feature.txt"))
	if readErr != nil {
		t.Fatalf("read feature.txt: %v", readErr)
	}
	if got := string(content); got != "uncommitted\n" {
		t.Fatalf("feature.txt = %q, want the uncommitted edit kept", got)
	}
	if head := gitCurrentBranch(t, dir); head != "main" {
		t.Fatalf("head = %q, want main", head)
	}
}

func TestCheckoutBranchInRepo_FetchFailureIsErrorNotSkip(t *testing.T) {
	dir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "nonexistent"))

	repo := &db.Repository{ID: 1, Alias: "repo1", Path: dir, DefaultBranch: "main"}
	_, skipReason, err := checkoutBranchInRepo(repo, "feature/x")
	if err == nil {
		t.Fatalf("expected an error for an unreachable origin, got skip reason %q", skipReason)
	}
	if skipReason != "" {
		t.Fatalf("skip reason = %q, want empty alongside the error", skipReason)
	}
}

func TestCheckoutBranchDryRunItems_PreservesOrder(t *testing.T) {
	repos := make([]*db.Repository, 8)
	for i := range repos {
		dir := initRepo(t)
		if i%2 == 0 {
			mustRunGit(t, dir, "branch", "feature/x")
		}
		repos[i] = &db.Repository{ID: int64(i + 1), Alias: fmt.Sprintf("repo%d", i), Path: dir, DefaultBranch: "main"}
	}

	items := checkoutBranchDryRunItems(repos, "feature/x")
	if len(items) != len(repos) {
		t.Fatalf("items = %d, want %d", len(items), len(repos))
	}
	for i, item := range items {
		if item.repo.Alias != repos[i].Alias {
			t.Fatalf("items[%d] = %s, want %s", i, item.repo.Alias, repos[i].Alias)
		}
		if missing := i%2 == 1; missing != (item.skipReason != "") {
			t.Fatalf("items[%d] (%s) skip reason = %q, want skipped=%v", i, item.repo.Alias, item.skipReason, missing)
		}
	}
}

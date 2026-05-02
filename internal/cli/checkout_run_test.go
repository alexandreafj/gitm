package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
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

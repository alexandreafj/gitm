package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

func TestRunCheckoutWithUI_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	ui := fakeUI{}

	if err := runCheckoutWithUI(ui, []string{""}); err != nil {
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

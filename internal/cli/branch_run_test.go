package cli

import (
	"testing"

	"github.com/alexandreferreira/gitm/internal/git"
)

func TestBranchCreate_SelectAll(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	repo2Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"feature/test"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}

	if !git.BranchExists(repo1Dir, "feature/test") {
		t.Fatal("expected feature/test to exist in repo1")
	}
	if !git.BranchExists(repo2Dir, "feature/test") {
		t.Fatal("expected feature/test to exist in repo2")
	}
}

func TestBranchCreate_UntrackedFilesAllowed(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	// Add an untracked file — should NOT block branch creation.
	writeFile(t, repoDir, "untracked.txt", "I am untracked\n")

	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"feature/untracked-test"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}

	if !git.BranchExists(repoDir, "feature/untracked-test") {
		t.Fatal("expected feature/untracked-test to exist despite untracked file")
	}
}

func TestBranchCreate_TrackedChangesBlocked(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	// Modify a tracked file — should still block branch creation.
	writeFile(t, repoDir, "README.md", "modified content\n")

	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"feature/should-be-skipped"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}

	if git.BranchExists(repoDir, "feature/should-be-skipped") {
		t.Fatal("expected branch creation to be skipped due to tracked changes")
	}
}

func TestBranchRename_NoReposWithBranch(t *testing.T) {
	database = setupTestDB(t)
	_, _ = newRepo(t, database, "repo1")

	cmd := branchRenameCmd()
	err := cmd.RunE(cmd, []string{"old", "new"})
	if err == nil {
		t.Fatal("expected error when no repos have branch")
	}
}

func TestBranchRename_NoRemote(t *testing.T) {
	database = setupTestDB(t)
	_, dir := newRepo(t, database, "repo1")
	mustRunGit(t, dir, "branch", "old")

	cmd := branchRenameCmd()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.Flags().Set("no-remote", "true"); err != nil {
		t.Fatalf("set flag no-remote: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"old", "new"}); err != nil {
		t.Fatalf("branch rename: %v", err)
	}

	if !git.BranchExists(dir, "new") {
		t.Fatal("expected new branch after rename")
	}
	if git.BranchExists(dir, "old") {
		t.Fatal("expected old branch to be gone")
	}
}

func TestBranchRename_Remote(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "checkout", "-b", "old")
	mustRunGit(t, repoDir, "push", "--set-upstream", "origin", "old")

	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	cmd := branchRenameCmd()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"old", "new"}); err != nil {
		t.Fatalf("branch rename: %v", err)
	}

	if git.RemoteBranchExists(repoDir, "old") {
		t.Fatal("expected old remote branch to be deleted")
	}
	if !git.RemoteBranchExists(repoDir, "new") {
		t.Fatal("expected new remote branch to exist")
	}
}

package cli

import (
	"testing"

	"github.com/alexandreferreira/gitm/internal/git"
)

func TestBranchCreate_SelectAll(t *testing.T) {
	database = setupTestDB(t)
	_, _ = newRepo(t, database, "repo1")
	_, _ = newRepo(t, database, "repo2")

	cmd := branchCreateCmd()
	cmd.Flags().Set("all", "true")
	if err := cmd.RunE(cmd, []string{"feature/test"}); err != nil {
		t.Fatalf("branch create: %v", err)
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
	cmd.Flags().Set("all", "true")
	cmd.Flags().Set("no-remote", "true")
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
	cmd.Flags().Set("all", "true")
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

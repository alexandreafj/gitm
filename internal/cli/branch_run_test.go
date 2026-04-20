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

func TestBranchCreate_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	repo2Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}
	repo3Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo3", "repo3", repo3Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("repo", "repo1,repo2"); err != nil {
		t.Fatalf("set flag repo: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"feature/AA-1"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}

	if !git.BranchExists(repo1Dir, "feature/AA-1") {
		t.Error("expected feature/AA-1 to exist in repo1")
	}
	if !git.BranchExists(repo2Dir, "feature/AA-1") {
		t.Error("expected feature/AA-1 to exist in repo2")
	}
	if git.BranchExists(repo3Dir, "feature/AA-1") {
		t.Error("expected feature/AA-1 NOT to exist in repo3 (not in --repo list)")
	}
}

func TestBranchCreate_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("repo", "repo1,does-not-exist"); err != nil {
		t.Fatalf("set flag repo: %v", err)
	}
	err := cmd.RunE(cmd, []string{"feature/AA-1"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
}

func TestBranchCreate_RepoFlag_TakesPrecedenceOverAll(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	repo2Dir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	// --repo repo1 + --all: only repo1 should get the branch.
	cmd := branchCreateCmd()
	if err := cmd.Flags().Set("repo", "repo1"); err != nil {
		t.Fatalf("set flag repo: %v", err)
	}
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set flag all: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"feature/AA-2"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}

	if !git.BranchExists(repo1Dir, "feature/AA-2") {
		t.Error("expected feature/AA-2 to exist in repo1")
	}
	if git.BranchExists(repo2Dir, "feature/AA-2") {
		t.Error("expected feature/AA-2 NOT to exist in repo2 (--repo takes precedence over --all)")
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

func TestBranchRename_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repo1Dir, "checkout", "-b", "old")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repo2Dir, "checkout", "-b", "old")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	repo3Dir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repo3Dir, "checkout", "-b", "old")
	if _, err := database.AddRepository("repo3", "repo3", repo3Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	cmd := branchRenameCmd()
	if err := cmd.Flags().Set("repo", "repo1,repo2"); err != nil {
		t.Fatalf("set flag repo: %v", err)
	}
	if err := cmd.Flags().Set("no-remote", "true"); err != nil {
		t.Fatalf("set flag no-remote: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"old", "new"}); err != nil {
		t.Fatalf("branch rename: %v", err)
	}

	if !git.BranchExists(repo1Dir, "new") {
		t.Error("expected branch 'new' to exist in repo1")
	}
	if git.BranchExists(repo1Dir, "old") {
		t.Error("expected branch 'old' to be gone from repo1")
	}
	if !git.BranchExists(repo2Dir, "new") {
		t.Error("expected branch 'new' to exist in repo2")
	}
	if git.BranchExists(repo2Dir, "old") {
		t.Error("expected branch 'old' to be gone from repo2")
	}
	// repo3 was NOT in --repo list — should be untouched.
	if git.BranchExists(repo3Dir, "new") {
		t.Error("expected branch 'new' NOT to exist in repo3 (not in --repo list)")
	}
	if !git.BranchExists(repo3Dir, "old") {
		t.Error("expected branch 'old' to still exist in repo3 (not in --repo list)")
	}
}

func TestBranchRename_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repo1Dir, "checkout", "-b", "old")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	cmd := branchRenameCmd()
	if err := cmd.Flags().Set("repo", "repo1,ghost-repo"); err != nil {
		t.Fatalf("set flag repo: %v", err)
	}
	if err := cmd.Flags().Set("no-remote", "true"); err != nil {
		t.Fatalf("set flag no-remote: %v", err)
	}
	err := cmd.RunE(cmd, []string{"old", "new"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
}

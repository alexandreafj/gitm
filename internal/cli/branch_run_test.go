package cli

import (
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
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

func TestBranchDelete_RepoFlag_LocalAndRemote(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "branch", "feature/x")
	mustRunGit(t, repoDir, "push", "origin", "feature/x")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/x", false, false, false, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if git.BranchExists(repoDir, "feature/x") {
		t.Error("expected local feature/x to be deleted")
	}
	if git.RemoteBranchExists(repoDir, "feature/x") {
		t.Error("expected remote feature/x to be deleted")
	}
}

func TestBranchDelete_NoRemote(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "branch", "feature/x")
	mustRunGit(t, repoDir, "push", "origin", "feature/x")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/x", false, false, true, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if git.BranchExists(repoDir, "feature/x") {
		t.Error("expected local feature/x to be deleted")
	}
	if !git.RemoteBranchExists(repoDir, "feature/x") {
		t.Error("expected remote feature/x to survive with --no-remote")
	}
}

func TestBranchDelete_Interactive(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "branch", "feature/x")
	mustRunGit(t, repoDir, "push", "origin", "feature/x")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// No --repo / --all: the interactive multi-select path. fakeUI returns
	// every offered repo, so feature/x should be deleted.
	if err := runBranchDeleteWithUI(fakeUI{}, "feature/x", false, false, false, nil); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if git.BranchExists(repoDir, "feature/x") {
		t.Error("expected feature/x to be deleted via interactive selection")
	}
}

func TestBranchDelete_ConfirmationDeclined(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "branch", "feature/x")
	mustRunGit(t, repoDir, "push", "origin", "feature/x")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: false}, "feature/x", false, false, false, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if !git.BranchExists(repoDir, "feature/x") {
		t.Error("expected feature/x to survive when confirmation is declined")
	}
	if !git.RemoteBranchExists(repoDir, "feature/x") {
		t.Error("expected remote feature/x to survive when confirmation is declined")
	}
}

func TestBranchDelete_DefaultBranchProtected(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "main", false, false, true, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if !git.BranchExists(repoDir, "main") {
		t.Error("expected the default branch main to be protected from deletion")
	}
}

func TestBranchDelete_CurrentBranchSkipped(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "checkout", "-b", "feature/current")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/current", false, false, true, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if !git.BranchExists(repoDir, "feature/current") {
		t.Error("expected the checked-out branch to be skipped, not deleted")
	}
}

func TestBranchDelete_UnmergedWithoutForceSkipped(t *testing.T) {
	database = setupTestDB(t)
	repoDir := unmergedBranchRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// Without --force, git refuses to drop a branch with unmerged commits.
	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/unmerged", false, false, true, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if !git.BranchExists(repoDir, "feature/unmerged") {
		t.Error("expected unmerged branch to survive deletion without --force")
	}
}

func TestBranchDelete_UnmergedWithForce(t *testing.T) {
	database = setupTestDB(t)
	repoDir := unmergedBranchRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/unmerged", false, true, true, []string{"repo1"}); err != nil {
		t.Fatalf("branch delete: %v", err)
	}

	if git.BranchExists(repoDir, "feature/unmerged") {
		t.Error("expected unmerged branch to be deleted with --force")
	}
}

func TestBranchDelete_NoReposWithBranch(t *testing.T) {
	database = setupTestDB(t)
	_, _ = newRepo(t, database, "repo1")

	err := runBranchDeleteWithUI(fakeUI{confirm: true}, "ghost", false, false, true, nil)
	if err == nil {
		t.Fatal("expected error when no repos have the branch")
	}
}

func TestBranchDelete_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	_, dir := newRepo(t, database, "repo1")
	mustRunGit(t, dir, "branch", "feature/x")

	err := runBranchDeleteWithUI(fakeUI{confirm: true}, "feature/x", false, false, true, []string{"repo1", "ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
}

// unmergedBranchRepo builds a repo whose feature/unmerged branch carries a
// commit not reachable from main, then returns to main.
func unmergedBranchRepo(t *testing.T) string {
	t.Helper()
	repoDir, _, _ := initRepoWithRemote(t)
	mustRunGit(t, repoDir, "checkout", "-b", "feature/unmerged")
	writeFile(t, repoDir, "extra.txt", "extra\n")
	mustRunGit(t, repoDir, "add", "extra.txt")
	mustRunGit(t, repoDir, "commit", "-m", "unmerged commit")
	mustRunGit(t, repoDir, "checkout", "main")
	return repoDir
}

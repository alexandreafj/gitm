package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreferreira/gitm/internal/git"
)

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRunGit(t, dir, "init", "--bare")
	return dir
}

func cloneRepo(t *testing.T, origin string) string {
	t.Helper()
	parent := t.TempDir()
	cloneDir := filepath.Join(parent, "clone")
	cmd := exec.Command("git", "clone", origin, cloneDir)
	cmd.Dir = parent
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	return cloneDir
}

func initRepoWithRemote(t *testing.T) (workDir, originDir string) {
	t.Helper()
	originDir = initBareRepo(t)
	workDir = initRepo(t)
	mustRunGit(t, workDir, "remote", "add", "origin", originDir)

	branch, err := git.CurrentBranch(workDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	mustRunGit(t, workDir, "push", "--set-upstream", "origin", branch)
	return workDir, originDir
}

func TestIsGitRepo(t *testing.T) {
	dir := initRepo(t)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if !git.IsGitRepo(resolved) {
		t.Errorf("expected %s to be a git repo", dir)
	}

	subdir := filepath.Join(resolved, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if git.IsGitRepo(subdir) {
		t.Errorf("expected %s to NOT be repo root", subdir)
	}

	emptyDir := t.TempDir()
	if git.IsGitRepo(emptyDir) {
		t.Errorf("expected %s to NOT be a git repo", emptyDir)
	}
}

func TestDefaultBranchUsesOriginHead(t *testing.T) {
	repo, origin := initRepoWithRemote(t)
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	mustRunGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)

	got, err := git.DefaultBranch(repo)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != branch {
		t.Errorf("DefaultBranch = %q, want %q (origin: %s)", got, branch, origin)
	}
}

func TestDefaultBranchFallsBackToHead(t *testing.T) {
	repo := initRepo(t)
	mustRunGit(t, repo, "branch", "-M", "develop")

	got, err := git.DefaultBranch(repo)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "develop" {
		t.Errorf("DefaultBranch = %q, want %q", got, "develop")
	}
}

func TestDirtyChecksAndLists(t *testing.T) {
	repo := initRepo(t)

	dirty, err := git.IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("expected clean repo")
	}

	writeFile(t, repo, "untracked.txt", "hi")
	dirty, err = git.IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo due to untracked file")
	}

	trackedDirty, err := git.IsDirtyTrackedOnly(repo)
	if err != nil {
		t.Fatalf("IsDirtyTrackedOnly: %v", err)
	}
	if trackedDirty {
		t.Error("expected tracked-only dirty to be false for untracked files")
	}

	writeFile(t, repo, "README.md", "changed\n")
	trackedDirty, err = git.IsDirtyTrackedOnly(repo)
	if err != nil {
		t.Fatalf("IsDirtyTrackedOnly: %v", err)
	}
	if !trackedDirty {
		t.Error("expected tracked-only dirty to be true after tracked change")
	}

	files, err := git.DirtyFiles(repo)
	if err != nil {
		t.Fatalf("DirtyFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected dirty files list to be non-empty")
	}

	withStatus, err := git.DirtyFilesWithStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithStatus: %v", err)
	}
	if len(withStatus) == 0 {
		t.Fatal("expected dirty files with status to be non-empty")
	}
	hasReadme := false
	for _, line := range withStatus {
		if strings.Contains(line, "README.md") {
			hasReadme = true
			break
		}
	}
	if !hasReadme {
		t.Error("expected README.md in DirtyFilesWithStatus")
	}
}

func TestCheckoutAndBranchOps(t *testing.T) {
	repo := initRepo(t)

	if err := git.CreateBranch(repo, "feature/test"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature/test" {
		t.Fatalf("CurrentBranch = %q, want %q", branch, "feature/test")
	}

	if err := git.Checkout(repo, "master"); err != nil {
		if err := git.Checkout(repo, "main"); err != nil {
			t.Fatalf("Checkout: %v", err)
		}
	}

	if !git.BranchExists(repo, "feature/test") {
		t.Error("expected BranchExists to return true")
	}
	if git.BranchExists(repo, "nope") {
		t.Error("expected BranchExists to return false")
	}
}

func TestRemoteBranchOps(t *testing.T) {
	repo, origin := initRepoWithRemote(t)
	_ = origin

	if err := git.CreateBranch(repo, "feature/remote"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := git.PushBranch(repo, "feature/remote"); err != nil {
		t.Fatalf("PushBranch: %v", err)
	}
	if !git.RemoteBranchExists(repo, "feature/remote") {
		t.Error("expected RemoteBranchExists to be true")
	}
	if git.RemoteBranchExists(repo, "missing") {
		t.Error("expected RemoteBranchExists to be false")
	}

	if err := git.DeleteRemoteBranch(repo, "feature/remote"); err != nil {
		t.Fatalf("DeleteRemoteBranch: %v", err)
	}
	if git.RemoteBranchExists(repo, "feature/remote") {
		t.Error("expected RemoteBranchExists to be false after delete")
	}
}

func TestRenameBranch(t *testing.T) {
	repo := initRepo(t)
	mustRunGit(t, repo, "branch", "old-name")

	if err := git.RenameBranch(repo, "old-name", "new-name"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if git.BranchExists(repo, "old-name") {
		t.Error("expected old branch to be gone")
	}
	if !git.BranchExists(repo, "new-name") {
		t.Error("expected new branch to exist")
	}
}

func TestPullAndPush(t *testing.T) {
	repo1, origin := initRepoWithRemote(t)
	repo2 := cloneRepo(t, origin)

	mustRunGit(t, repo2, "config", "user.email", "test@example.com")
	mustRunGit(t, repo2, "config", "user.name", "Test User")
	writeFile(t, repo2, "from-remote.txt", "remote\n")
	mustRunGit(t, repo2, "add", "from-remote.txt")
	mustRunGit(t, repo2, "commit", "-m", "remote commit")
	mustRunGit(t, repo2, "push")

	if _, err := git.Pull(repo1); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo1, "from-remote.txt")); err != nil {
		t.Fatalf("expected pulled file to exist: %v", err)
	}

	writeFile(t, repo1, "local.txt", "local\n")
	mustRunGit(t, repo1, "add", "local.txt")
	if _, err := git.Commit(repo1, "local commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := git.Push(repo1); err != nil {
		t.Fatalf("Push: %v", err)
	}
}

func TestAheadBehind(t *testing.T) {
	repo1, origin := initRepoWithRemote(t)
	repo2 := cloneRepo(t, origin)

	mustRunGit(t, repo2, "config", "user.email", "test@example.com")
	mustRunGit(t, repo2, "config", "user.name", "Test User")
	writeFile(t, repo2, "remote.txt", "remote\n")
	mustRunGit(t, repo2, "add", "remote.txt")
	mustRunGit(t, repo2, "commit", "-m", "remote commit")
	mustRunGit(t, repo2, "push")

	writeFile(t, repo1, "local.txt", "local\n")
	mustRunGit(t, repo1, "add", "local.txt")
	mustRunGit(t, repo1, "commit", "-m", "local commit")

	ahead, behind, err := git.AheadBehind(repo1, true)
	if err != nil {
		t.Fatalf("AheadBehind: %v", err)
	}
	if ahead == 0 || behind == 0 {
		t.Errorf("expected both ahead and behind to be > 0, got ahead=%d behind=%d", ahead, behind)
	}
}

func TestStageFilesAndDiscard(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", "one\n")
	writeFile(t, repo, "b.txt", "two\n")

	dirty, err := git.DirtyFilesWithStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithStatus: %v", err)
	}
	if err := git.StageFiles(repo, dirty); err != nil {
		t.Fatalf("StageFiles: %v", err)
	}
	staged := stagedFiles(t, repo)
	if len(staged) == 0 {
		t.Fatal("expected staged files after StageFiles")
	}

	if err := git.ResetMixed(repo, "HEAD"); err != nil {
		t.Fatalf("ResetMixed: %v", err)
	}

	if err := git.DiscardChanges(repo); err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}
	clean, err := git.IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if clean {
		t.Error("expected repo to be clean after DiscardChanges")
	}
	if _, err := os.Stat(filepath.Join(repo, "a.txt")); err == nil {
		t.Error("expected untracked file to be removed after DiscardChanges")
	}
}

func TestIsDefaultBranch(t *testing.T) {
	repo := initRepo(t)
	branch, err := git.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	ok, err := git.IsDefaultBranch(repo, branch)
	if err != nil {
		t.Fatalf("IsDefaultBranch: %v", err)
	}
	if !ok {
		t.Error("expected IsDefaultBranch to be true")
	}
	ok, err = git.IsDefaultBranch(repo, "other")
	if err != nil {
		t.Fatalf("IsDefaultBranch: %v", err)
	}
	if ok {
		t.Error("expected IsDefaultBranch to be false for other branch")
	}
}

func TestStashOps(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "stash.txt", "stash\n")

	if err := git.StashPush(repo, "test stash"); err != nil {
		t.Fatalf("StashPush: %v", err)
	}

	has, err := git.HasStash(repo)
	if err != nil {
		t.Fatalf("HasStash: %v", err)
	}
	if !has {
		t.Fatal("expected stash to exist")
	}

	entries, err := git.StashList(repo)
	if err != nil {
		t.Fatalf("StashList: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected stash entries")
	}

	if err := git.StashApply(repo); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	has, err = git.HasStash(repo)
	if err != nil {
		t.Fatalf("HasStash: %v", err)
	}
	if !has {
		t.Error("expected stash to remain after apply")
	}

	mustRunGit(t, repo, "reset", "--hard")
	mustRunGit(t, repo, "clean", "-fd")

	if err := git.StashPop(repo); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	has, err = git.HasStash(repo)
	if err != nil {
		t.Fatalf("HasStash: %v", err)
	}
	if has {
		t.Error("expected stash to be removed after pop")
	}
}

func TestRepoName(t *testing.T) {
	got := git.RepoName("/tmp/some/repo")
	if got != "repo" {
		t.Errorf("RepoName = %q, want %q", got, "repo")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	_ = os.RemoveAll(temp)
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("Chdir back to cwd: %v", err)
		}
	}()

	got = git.RepoName("foo/bar")
	if got != "bar" {
		t.Errorf("RepoName(rel) = %q, want %q", got, "bar")
	}
}

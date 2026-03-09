package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
)

type fakeUI struct {
	selectRepos []*db.Repository
	selectErr   error
	fileErr     error
	fileSelect  []string
	commitErr   error
	commitMsg   string
	branchSeen  *string
	branchErr   error
	branchName  string
}

func (f fakeUI) FileSelect(porcelainLines []string, title string) ([]string, error) {
	if f.fileErr != nil {
		return nil, f.fileErr
	}
	if f.fileSelect != nil {
		return f.fileSelect, nil
	}
	return porcelainLines, nil
}

func (f fakeUI) MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error) {
	if f.selectErr != nil {
		return nil, f.selectErr
	}
	if f.selectRepos != nil {
		return f.selectRepos, nil
	}
	return repos, nil
}

func (f fakeUI) CommitMessageInput(repoAlias, branchName string) (string, error) {
	if f.commitErr != nil {
		return "", f.commitErr
	}
	if f.branchSeen != nil {
		*f.branchSeen = branchName
	}
	if f.commitMsg != "" {
		return f.commitMsg, nil
	}
	return "test commit", nil
}

func (f fakeUI) BranchNameInput() (string, error) {
	if f.branchErr != nil {
		return "", f.branchErr
	}
	if f.branchName != "" {
		return f.branchName, nil
	}
	return "feature/test", nil
}

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	database = d
	t.Cleanup(func() {
		_ = d.Close()
		database = nil
	})
	return d
}

func newRepo(t *testing.T, database *db.DB, alias string) (*db.Repository, string) {
	t.Helper()
	dir := initRepo(t)
	r, err := database.AddRepository(alias, alias, dir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	return r, dir
}

func mustRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimRight(string(out), "\r\n")
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test User")
	mustRunGit(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, dir, "README.md", "# test repo\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "initial commit")
	return dir
}

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

func initRepoWithRemote(t *testing.T) (repoDir, originDir, branch string) {
	t.Helper()
	originDir = initBareRepo(t)
	repoDir = initRepo(t)
	mustRunGit(t, repoDir, "remote", "add", "origin", originDir)
	branch = gitCurrentBranch(t, repoDir)
	mustRunGit(t, repoDir, "push", "--set-upstream", "origin", branch)
	return repoDir, originDir, branch
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

func gitCurrentBranch(t *testing.T, dir string) string {
	t.Helper()
	branch, err := git.CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	return branch
}

func gitCreateBranch(t *testing.T, dir, name string) {
	t.Helper()
	if err := git.CreateBranch(dir, name); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
}

package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

type fakeUI struct {
	selectRepos     []*db.Repository
	selectErr       error
	multiSelectHook func()
	fileErr         error
	fileSelect      []string
	commitErr       error
	commitMsg       string
	branchSeen      *string
	branchErr       error
	branchName      string
	confirm         bool
	confirmErr      error
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
	if f.multiSelectHook != nil {
		f.multiSelectHook()
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

func (f fakeUI) Confirm(prompt string) (bool, error) {
	if f.confirmErr != nil {
		return false, f.confirmErr
	}
	return f.confirm, nil
}

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := os.WriteFile(dbPath, templates.db, 0644); err != nil {
		t.Fatalf("write template db: %v", err)
	}
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
	initRepoAt(t, dir)
	return dir
}

func initRepoAt(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	copyRepoTemplate(t, templates.repo, dir)
}

// copyRepoTemplate copies a template work tree and refreshes its index: the
// copied index still carries the template files' inode and ctime, which
// plumbing such as diff-files or diff-index would report as modifications.
func copyRepoTemplate(t *testing.T, template, dir string) {
	t.Helper()
	if err := os.CopyFS(dir, os.DirFS(template)); err != nil {
		t.Fatalf("copy repo template to %s: %v", dir, err)
	}
	mustRunGit(t, dir, "update-index", "-q", "--refresh")
}

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRunGit(t, dir, "init", "--bare", "--initial-branch=main")
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
	originDir = t.TempDir()
	if err := os.CopyFS(originDir, os.DirFS(templates.origin)); err != nil {
		t.Fatalf("copy origin template: %v", err)
	}
	repoDir = t.TempDir()
	copyRepoTemplate(t, templates.remoteRepo, repoDir)
	mustRunGit(t, repoDir, "remote", "set-url", "origin", originDir)
	return repoDir, originDir, templateBranch
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

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
		_ = r.Close()
		_ = w.Close()
	}()

	fn()

	_ = w.Close()

	var output bytes.Buffer
	if _, err := io.Copy(&output, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}

	return output.String()
}

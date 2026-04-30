package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// initRepo creates a temporary, fully-initialized git repo with an initial
// commit so that HEAD exists and all operations are valid.
func initRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	mustGit("init", "-b", "main")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test User")
	mustGit("config", "commit.gpgsign", "false")

	// Initial commit so HEAD is valid.
	writeFile(t, dir, "README.md", "# test repo\n")
	mustGit("add", ".")
	mustGit("commit", "-m", "initial commit")

	return dir
}

// makeCommit stages and commits a file with the given name and content.
func makeCommit(t *testing.T, dir, filename, content, message string) {
	t.Helper()
	writeFile(t, dir, filename, content)
	mustRunGit(t, dir, "add", filename)
	mustRunGit(t, dir, "commit", "-m", message)
}

// writeFile creates (or overwrites) a file inside dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

// mustRunGit runs a git command in dir and fails the test on error.
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

// readFileContent reads a file from the repo directory.
func readFileContent(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("readFile %s: %v", name, err)
	}
	return string(data)
}

// stagedFiles returns the list of files that are staged (index has changes vs HEAD).
func stagedFiles(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff --cached: %v\n%s", err, out)
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files
}

// unstagedModifiedFiles returns files that appear in the working tree with
// changes (modified or untracked) but are NOT staged. It uses git status
// --porcelain so that both modified tracked files and new untracked files
// are captured — a plain `git diff` only shows tracked changes.
func unstagedModifiedFiles(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain: %v\n%s", err, out)
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Porcelain format: "XY filename"
		// X = index status, Y = worktree status
		// We want entries where the worktree column (Y, pos 1) is non-space,
		// OR the file is completely untracked ("??").
		if len(l) < 3 {
			continue
		}
		xy := l[:2]
		filename := strings.TrimSpace(l[3:])
		// Skip entries that are only staged (X set, Y is space).
		if xy[0] != ' ' && xy[0] != '?' && xy[1] == ' ' {
			continue
		}
		files = append(files, filename)
	}
	return files
}

// ─── ResetSoft ──────────────────────────────────────────────────────────────

func TestResetSoft(t *testing.T) {
	dir := initRepo(t)

	// Make a commit on top of the initial one.
	makeCommit(t, dir, "feature.go", "package main\n", "add feature")

	// Verify we have 2 commits.
	log, err := git.CommitLog(dir, 5)
	if err != nil {
		t.Fatalf("CommitLog: %v", err)
	}
	if len(log) < 2 {
		t.Fatalf("expected at least 2 commits, got %d", len(log))
	}

	// Soft reset — HEAD should move back, but changes must remain staged.
	if err := git.ResetSoft(dir, "HEAD~1"); err != nil {
		t.Fatalf("ResetSoft: %v", err)
	}

	// After --soft, the file should appear in the staged index.
	staged := stagedFiles(t, dir)
	found := false
	for _, f := range staged {
		if f == "feature.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected feature.go to be staged after soft reset; staged files: %v", staged)
	}

	// Working-tree content must still be present.
	content := readFileContent(t, dir, "feature.go")
	if !strings.Contains(content, "package main") {
		t.Errorf("expected file content to survive soft reset, got: %q", content)
	}
}

// ─── ResetMixed ─────────────────────────────────────────────────────────────

func TestResetMixed(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "service.go", "package service\n", "add service")

	if err := git.ResetMixed(dir, "HEAD~1"); err != nil {
		t.Fatalf("ResetMixed: %v", err)
	}

	// After mixed reset the file must NOT be staged.
	staged := stagedFiles(t, dir)
	for _, f := range staged {
		if f == "service.go" {
			t.Errorf("service.go should NOT be staged after mixed reset; staged files: %v", staged)
		}
	}

	// But the file must still exist in the working tree.
	unstaged := unstagedModifiedFiles(t, dir)
	found := false
	for _, f := range unstaged {
		if f == "service.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected service.go to be an unstaged modification after mixed reset; got: %v", unstaged)
	}
}

// ─── ResetHard ──────────────────────────────────────────────────────────────

func TestResetHard(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "temp.go", "package temp\n", "add temp")

	if err := git.ResetHard(dir, "HEAD~1"); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}

	// After hard reset, the file must not exist at all.
	if _, err := os.Stat(filepath.Join(dir, "temp.go")); err == nil {
		t.Error("temp.go should have been removed by hard reset, but it still exists")
	}

	// Index must be clean.
	staged := stagedFiles(t, dir)
	if len(staged) != 0 {
		t.Errorf("expected empty index after hard reset, got: %v", staged)
	}

	// Working tree must be clean.
	unstaged := unstagedModifiedFiles(t, dir)
	if len(unstaged) != 0 {
		t.Errorf("expected clean working tree after hard reset, got: %v", unstaged)
	}
}

// ─── ResetHard multi-commit ─────────────────────────────────────────────────

func TestResetHardMultipleCommits(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "a.go", "a\n", "commit A")
	makeCommit(t, dir, "b.go", "b\n", "commit B")
	makeCommit(t, dir, "c.go", "c\n", "commit C")

	if err := git.ResetHard(dir, "HEAD~3"); err != nil {
		t.Fatalf("ResetHard HEAD~3: %v", err)
	}

	for _, f := range []string{"a.go", "b.go", "c.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("%s should not exist after 3-commit hard reset", f)
		}
	}
}

// ─── CommitLog ──────────────────────────────────────────────────────────────

func TestCommitLog(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "one.go", "1\n", "first extra commit")
	makeCommit(t, dir, "two.go", "2\n", "second extra commit")

	t.Run("returns requested count", func(t *testing.T) {
		log, err := git.CommitLog(dir, 2)
		if err != nil {
			t.Fatalf("CommitLog: %v", err)
		}
		if len(log) != 2 {
			t.Errorf("expected 2 entries, got %d: %v", len(log), log)
		}
	})

	t.Run("most recent commit is first", func(t *testing.T) {
		log, err := git.CommitLog(dir, 2)
		if err != nil {
			t.Fatalf("CommitLog: %v", err)
		}
		if len(log) < 2 {
			t.Fatalf("need at least 2 log entries")
		}
		if !strings.Contains(log[0], "second extra commit") {
			t.Errorf("expected first entry to be the newest commit, got: %q", log[0])
		}
		if !strings.Contains(log[1], "first extra commit") {
			t.Errorf("expected second entry to be the older commit, got: %q", log[1])
		}
	})

	t.Run("entries have short hash prefix", func(t *testing.T) {
		log, err := git.CommitLog(dir, 1)
		if err != nil {
			t.Fatalf("CommitLog: %v", err)
		}
		if len(log) == 0 {
			t.Fatal("expected at least one log entry")
		}
		// Oneline format is "<7-char-hash> <subject>"
		parts := strings.SplitN(log[0], " ", 2)
		if len(parts[0]) < 7 {
			t.Errorf("expected short hash (>=7 chars) at start of log entry, got: %q", log[0])
		}
	})

	t.Run("n=1 returns exactly one entry", func(t *testing.T) {
		log, err := git.CommitLog(dir, 1)
		if err != nil {
			t.Fatalf("CommitLog(1): %v", err)
		}
		if len(log) != 1 {
			t.Errorf("expected 1 entry, got %d", len(log))
		}
	})

	t.Run("requesting more than available returns what exists", func(t *testing.T) {
		log, err := git.CommitLog(dir, 100)
		if err != nil {
			t.Fatalf("CommitLog(100): %v", err)
		}
		// We made 3 commits in total (initial + 2 extras), so <= 100.
		if len(log) == 0 {
			t.Error("expected at least one log entry")
		}
		if len(log) > 100 {
			t.Errorf("log returned more than 100 entries: %d", len(log))
		}
	})
}

// ─── ResetSoft preserves content through multiple commits ────────────────────

func TestResetSoftMultipleCommits(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "alpha.go", "alpha\n", "alpha commit")
	makeCommit(t, dir, "beta.go", "beta\n", "beta commit")

	if err := git.ResetSoft(dir, "HEAD~2"); err != nil {
		t.Fatalf("ResetSoft HEAD~2: %v", err)
	}

	// Both files should be staged.
	staged := stagedFiles(t, dir)
	stagedSet := make(map[string]bool)
	for _, f := range staged {
		stagedSet[f] = true
	}

	if !stagedSet["alpha.go"] {
		t.Errorf("alpha.go should be staged after soft 2-commit reset; staged: %v", staged)
	}
	if !stagedSet["beta.go"] {
		t.Errorf("beta.go should be staged after soft 2-commit reset; staged: %v", staged)
	}
}

// ─── ForcePush (local bare remote) ──────────────────────────────────────────

// TestForcePush sets up a local bare repository as "origin" to exercise the
// ForcePush code path without requiring network access.
func TestForcePush(t *testing.T) {
	// Create a bare repo to act as origin.
	bareDir := t.TempDir()
	mustRunGit(t, bareDir, "init", "--bare", "--initial-branch=main")

	// Create a working repo and push an initial commit.
	workDir := initRepo(t)
	mustRunGit(t, workDir, "config", "user.email", "test@example.com")
	mustRunGit(t, workDir, "config", "user.name", "Test User")
	mustRunGit(t, workDir, "remote", "add", "origin", bareDir)

	// Detect the actual branch name (main vs master differs by git version/config).
	branch, err := git.CurrentBranch(workDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	mustRunGit(t, workDir, "push", "--set-upstream", "origin", branch)

	// Make a new local commit then push it.
	makeCommit(t, workDir, "pushed.go", "pushed\n", "pushed commit")
	mustRunGit(t, workDir, "push", "origin", branch)

	// Now locally undo that commit with a soft reset.
	if resetErr := git.ResetSoft(workDir, "HEAD~1"); resetErr != nil {
		t.Fatalf("ResetSoft: %v", resetErr)
	}

	// Re-commit with a different message — now local history has diverged from origin.
	mustRunGit(t, workDir, "commit", "-m", "rewritten commit")

	// ForcePush should succeed.
	if pushErr := git.ForcePush(workDir); pushErr != nil {
		t.Fatalf("ForcePush: %v", pushErr)
	}

	// Verify origin now has the rewritten commit by cloning the bare repo.
	cloneDir := t.TempDir()
	mustRunGit(t, cloneDir, "clone", bareDir, ".")
	log, logErr := git.CommitLog(cloneDir, 1)
	if logErr != nil {
		t.Fatalf("CommitLog on clone: %v", logErr)
	}
	if len(log) == 0 {
		t.Fatal("expected at least one commit in cloned repo")
	}
	if !strings.Contains(log[0], "rewritten commit") {
		t.Errorf("expected rewritten commit on remote, got: %q", log[0])
	}
}

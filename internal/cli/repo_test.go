package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

// ─── TestPrintRepoTable ─────────────────────────────────────────────────────

// TestPrintRepoTableHandlesEmpty verifies that printRepoTable doesn't panic with empty list.
func TestPrintRepoTableHandlesEmpty(t *testing.T) {
	// This test ensures the function doesn't crash with an empty repository list.
	// We can't easily test the actual output without capturing stdout.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRepoTable panicked with empty repos: %v", r)
		}
	}()

	repos := []*db.Repository{}
	printRepoTable(repos)
}

// TestPrintRepoTableHandlesMultipleRepos verifies the function handles multiple repos.
func TestPrintRepoTableHandlesMultipleRepos(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRepoTable panicked with multiple repos: %v", r)
		}
	}()

	repos := []*db.Repository{
		{
			ID:            1,
			Name:          "api",
			Alias:         "api-gateway",
			Path:          "/home/user/api",
			DefaultBranch: "main",
		},
		{
			ID:            2,
			Name:          "web",
			Alias:         "web-ui",
			Path:          "/home/user/web",
			DefaultBranch: "master",
		},
	}
	printRepoTable(repos)
}

func TestRepoAddCmdAliasValidation(t *testing.T) {
	cmd := repoAddCmd()
	if err := cmd.Flags().Set("alias", "alias"); err != nil {
		t.Fatalf("set flag alias: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"/tmp/a", "/tmp/b"}); err == nil {
		t.Fatal("expected error when alias used with multiple paths")
	}
}

// ─── TestRepoAddAutoDetect ──────────────────────────────────────────────────

// TestRepoAddAutoDetect verifies that --auto-detect scans immediate children,
// registers git repos, and skips plain directories.
func TestRepoAddAutoDetect(t *testing.T) {
	d := setupTestDB(t)

	// Build a parent directory with:
	//   parent/
	//     repo-a/   ← real git repo
	//     repo-b/   ← real git repo
	//     not-a-repo/ ← plain directory (no .git)
	parent := t.TempDir()

	repoA := filepath.Join(parent, "repo-a")
	repoB := filepath.Join(parent, "repo-b")
	plain := filepath.Join(parent, "not-a-repo")

	for _, dir := range []string{repoA, repoB, plain} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	// Initialize the two git repos using the real git binary.
	for _, dir := range []string{repoA, repoB} {
		mustRunGit(t, dir, "init", "-b", "main")
		mustRunGit(t, dir, "config", "user.email", "test@example.com")
		mustRunGit(t, dir, "config", "user.name", "Test User")
		mustRunGit(t, dir, "config", "commit.gpgsign", "false")
		writeFile(t, dir, "README.md", "# test\n")
		mustRunGit(t, dir, "add", ".")
		mustRunGit(t, dir, "commit", "-m", "init")
	}

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd.RunE(cmd, []string{parent}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 registered repos, got %d", len(repos))
	}

	// Verify both repo-a and repo-b were registered by alias (directory name).
	aliases := map[string]bool{}
	for _, r := range repos {
		aliases[r.Alias] = true
	}
	for _, want := range []string{"repo-a", "repo-b"} {
		if !aliases[want] {
			t.Errorf("expected alias %q to be registered, got %v", want, aliases)
		}
	}
}

// TestRepoAddAutoDetectSkipsAlreadyRegistered verifies that repos already in
// the database are reported as skipped (⚠) and do not cause a hard failure.
func TestRepoAddAutoDetectSkipsAlreadyRegistered(t *testing.T) {
	d := setupTestDB(t)

	parent := t.TempDir()
	repoA := filepath.Join(parent, "repo-a")
	repoB := filepath.Join(parent, "repo-b")

	for _, dir := range []string{repoA, repoB} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		mustRunGit(t, dir, "init", "-b", "main")
		mustRunGit(t, dir, "config", "user.email", "test@example.com")
		mustRunGit(t, dir, "config", "user.name", "Test User")
		mustRunGit(t, dir, "config", "commit.gpgsign", "false")
		writeFile(t, dir, "README.md", "# test\n")
		mustRunGit(t, dir, "add", ".")
		mustRunGit(t, dir, "commit", "-m", "init")
	}

	// Pre-register repo-a so it is already in the DB.
	if _, err := d.AddRepository("repo-a", "repo-a", repoA, "main"); err != nil {
		t.Fatalf("pre-register repo-a: %v", err)
	}

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	// Should succeed (not return an error) even though repo-a is a duplicate.
	if err := cmd.RunE(cmd, []string{parent}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	// Both repos should be registered (repo-a was already there, repo-b is new).
	if len(repos) != 2 {
		t.Fatalf("expected 2 registered repos, got %d", len(repos))
	}
}

// TestRepoAddAutoDetectRejectsAlias verifies that combining --auto-detect with
// --alias returns an error, since a single alias cannot apply to many repos.
func TestRepoAddAutoDetectRejectsAlias(t *testing.T) {
	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag auto-detect: %v", err)
	}
	if err := cmd.Flags().Set("alias", "my-alias"); err != nil {
		t.Fatalf("set flag alias: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"/tmp/some-dir"}); err == nil {
		t.Fatal("expected error when --auto-detect and --alias are combined")
	}
}

// TestRepoAddAutoDetectRejectsMultiplePaths verifies that --auto-detect with
// more than one path argument returns an error.
func TestRepoAddAutoDetectRejectsMultiplePaths(t *testing.T) {
	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd.RunE(cmd, []string{"/tmp/a", "/tmp/b"}); err == nil {
		t.Fatal("expected error when --auto-detect is given multiple paths")
	}
}

// TestRepoAddAutoDetectNotADirectory verifies that passing a file path (not a
// directory) to --auto-detect returns a descriptive error.
func TestRepoAddAutoDetectNotADirectory(t *testing.T) {
	// Create a real file to pass as the "parent" path.
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	setupTestDB(t)

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd.RunE(cmd, []string{filePath}); err == nil {
		t.Fatal("expected error when path is a file, not a directory")
	}
}

// TestRepoAddAutoDetectSkipsHiddenDirs verifies that hidden directories
// (names starting with ".") are not scanned.
func TestRepoAddAutoDetectSkipsHiddenDirs(t *testing.T) {
	d := setupTestDB(t)

	parent := t.TempDir()
	hiddenRepo := filepath.Join(parent, ".hidden-repo")

	if err := os.MkdirAll(hiddenRepo, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	mustRunGit(t, hiddenRepo, "init", "-b", "main")
	mustRunGit(t, hiddenRepo, "config", "user.email", "test@example.com")
	mustRunGit(t, hiddenRepo, "config", "user.name", "Test User")
	mustRunGit(t, hiddenRepo, "config", "commit.gpgsign", "false")
	writeFile(t, hiddenRepo, "README.md", "# hidden\n")
	mustRunGit(t, hiddenRepo, "add", ".")
	mustRunGit(t, hiddenRepo, "commit", "-m", "init")

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd.RunE(cmd, []string{parent}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos (hidden dir skipped), got %d", len(repos))
	}
}

// ─── TestDiscoverRepos ──────────────────────────────────────────────────────

// TestDiscoverReposEmptyDir verifies that an empty directory returns no repos.
func TestDiscoverReposEmptyDir(t *testing.T) {
	parent := t.TempDir()
	repos, err := discoverRepos(parent)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

// TestDiscoverReposFindsGitRepos verifies that discoverRepos returns only git
// repo paths and ignores plain directories.
func TestDiscoverReposFindsGitRepos(t *testing.T) {
	parent := t.TempDir()

	gitDir := filepath.Join(parent, "my-service")
	plainDir := filepath.Join(parent, "docs")

	for _, dir := range []string{gitDir, plainDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	mustRunGit(t, gitDir, "init", "-b", "main")
	mustRunGit(t, gitDir, "config", "user.email", "test@example.com")
	mustRunGit(t, gitDir, "config", "user.name", "Test User")
	mustRunGit(t, gitDir, "config", "commit.gpgsign", "false")
	writeFile(t, gitDir, "main.go", "package main\n")
	mustRunGit(t, gitDir, "add", ".")
	mustRunGit(t, gitDir, "commit", "-m", "init")

	repos, err := discoverRepos(parent)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d: %v", len(repos), repos)
	}
	if repos[0] != gitDir {
		t.Errorf("expected %q, got %q", gitDir, repos[0])
	}
}

// TestDiscoverReposNonExistentPath verifies that a non-existent path returns
// a descriptive error.
func TestDiscoverReposNonExistentPath(t *testing.T) {
	_, err := discoverRepos("/this/path/does/not/exist/at/all")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

// TestDiscoverReposFileNotDir verifies that passing a file (not a directory)
// returns a descriptive error.
func TestDiscoverReposFileNotDir(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := discoverRepos(f)
	if err == nil {
		t.Fatal("expected error when path is a file")
	}
}

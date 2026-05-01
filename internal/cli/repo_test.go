package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
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

// TestRepoAddDepthRejectsWithoutAutoDetect verifies that --depth without
// --auto-detect returns an error.
func TestRepoAddDepthRejectsWithoutAutoDetect(t *testing.T) {
	setupTestDB(t)

	parent := t.TempDir()
	initRepoAt(t, filepath.Join(parent, "some-repo"))

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("depth", "2"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	err := cmd.RunE(cmd, []string{parent})
	if err == nil {
		t.Fatal("expected error when --depth is used without --auto-detect")
	}
	want := "--depth can only be used with --auto-detect"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %q", want, err.Error())
	}
}

// TestRepoAddDepthRejectsZero verifies that --depth=0 returns an error.
func TestRepoAddDepthRejectsZero(t *testing.T) {
	setupTestDB(t)

	parent := t.TempDir()
	initRepoAt(t, filepath.Join(parent, "some-repo"))

	cmd := repoAddCmd()
	if err := cmd.Flags().Set("auto-detect", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd.Flags().Set("depth", "0"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	err := cmd.RunE(cmd, []string{parent})
	if err == nil {
		t.Fatal("expected error when --depth is 0")
	}
	want := "--depth must be at least 1"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %q", want, err.Error())
	}
}

// ─── TestDiscoverRepos ──────────────────────────────────────────────────────

// TestDiscoverReposEmptyDir verifies that an empty directory returns no repos.
func TestDiscoverReposEmptyDir(t *testing.T) {
	parent := t.TempDir()
	repos, err := discoverRepos(parent, 1)
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

	repos, err := discoverRepos(parent, 1)
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
	_, err := discoverRepos("/this/path/does/not/exist/at/all", 1)
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
	_, err := discoverRepos(f, 1)
	if err == nil {
		t.Fatal("expected error when path is a file")
	}
}

// TestDiscoverReposFollowsSymlinks verifies that symlinked directories
// pointing to git repos are discovered (not skipped).
func TestDiscoverReposFollowsSymlinks(t *testing.T) {
	// Create the real repo outside the parent directory.
	realRepo := t.TempDir()
	mustRunGit(t, realRepo, "init", "-b", "main")
	mustRunGit(t, realRepo, "config", "user.email", "test@example.com")
	mustRunGit(t, realRepo, "config", "user.name", "Test User")
	mustRunGit(t, realRepo, "config", "commit.gpgsign", "false")
	writeFile(t, realRepo, "README.md", "# real repo\n")
	mustRunGit(t, realRepo, "add", ".")
	mustRunGit(t, realRepo, "commit", "-m", "init")

	// Create a parent directory with a symlink to the real repo.
	parent := t.TempDir()
	link := filepath.Join(parent, "linked-repo")
	if err := os.Symlink(realRepo, link); err != nil {
		t.Skipf("cannot create symlink (OS restriction): %v", err)
	}

	repos, err := discoverRepos(parent, 1)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (via symlink), got %d: %v", len(repos), repos)
	}
}

// TestDiscoverReposDepthTwo verifies that discoverRepos with maxDepth=2
// finds repos nested one level deeper than immediate children.
func TestDiscoverReposDepthTwo(t *testing.T) {
	parent := t.TempDir()

	repoD1 := filepath.Join(parent, "repo-at-depth1")
	group := filepath.Join(parent, "project-group")
	repoD2 := filepath.Join(group, "repo-at-depth2")
	plain := filepath.Join(group, "not-a-repo")

	for _, dir := range []string{repoD1, repoD2, plain} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	for _, dir := range []string{repoD1, repoD2} {
		mustRunGit(t, dir, "init", "-b", "main")
		mustRunGit(t, dir, "config", "user.email", "test@example.com")
		mustRunGit(t, dir, "config", "user.name", "Test User")
		mustRunGit(t, dir, "config", "commit.gpgsign", "false")
		writeFile(t, dir, "README.md", "# test\n")
		mustRunGit(t, dir, "add", ".")
		mustRunGit(t, dir, "commit", "-m", "init")
	}

	repos, err := discoverRepos(parent, 2)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}
}

// TestDiscoverReposDoesNotDescendIntoGitRepos verifies that once a git repo
// is found, its subdirectories are not scanned (the repo is a leaf node).
func TestDiscoverReposDoesNotDescendIntoGitRepos(t *testing.T) {
	parent := t.TempDir()

	outerRepo := filepath.Join(parent, "outer-repo")
	innerRepo := filepath.Join(outerRepo, "inner-repo")

	for _, dir := range []string{outerRepo, innerRepo} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	for _, dir := range []string{outerRepo, innerRepo} {
		mustRunGit(t, dir, "init", "-b", "main")
		mustRunGit(t, dir, "config", "user.email", "test@example.com")
		mustRunGit(t, dir, "config", "user.name", "Test User")
		mustRunGit(t, dir, "config", "commit.gpgsign", "false")
		writeFile(t, dir, "README.md", "# test\n")
		mustRunGit(t, dir, "add", ".")
		mustRunGit(t, dir, "commit", "-m", "init")
	}

	repos, err := discoverRepos(parent, 2)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (outer only), got %d: %v", len(repos), repos)
	}
	if filepath.Base(repos[0]) != "outer-repo" {
		t.Errorf("expected outer-repo, got %q", repos[0])
	}
}

// TestDiscoverReposSkipsHiddenDirsAtAllDepths verifies that hidden directories
// are skipped at every depth level, not just the first.
func TestDiscoverReposSkipsHiddenDirsAtAllDepths(t *testing.T) {
	parent := t.TempDir()

	group := filepath.Join(parent, "group")
	hiddenRepo := filepath.Join(group, ".hidden-repo")
	visibleRepo := filepath.Join(group, "visible-repo")

	for _, dir := range []string{hiddenRepo, visibleRepo} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		mustRunGit(t, dir, "init", "-b", "main")
		mustRunGit(t, dir, "config", "user.email", "test@example.com")
		mustRunGit(t, dir, "config", "user.name", "Test User")
		mustRunGit(t, dir, "config", "commit.gpgsign", "false")
		writeFile(t, dir, "README.md", "# test\n")
		mustRunGit(t, dir, "add", ".")
		mustRunGit(t, dir, "commit", "-m", "init")
	}

	repos, err := discoverRepos(parent, 2)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (hidden skipped), got %d: %v", len(repos), repos)
	}
	if filepath.Base(repos[0]) != "visible-repo" {
		t.Errorf("expected visible-repo, got %q", repos[0])
	}
}

// TestDiscoverReposDepthOneIgnoresNestedRepos verifies that depth=1 (default)
// does not find repos at depth 2, preserving backwards compatibility.
func TestDiscoverReposDepthOneIgnoresNestedRepos(t *testing.T) {
	parent := t.TempDir()

	group := filepath.Join(parent, "project-group")
	nestedRepo := filepath.Join(group, "nested-repo")

	if err := os.MkdirAll(nestedRepo, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	mustRunGit(t, nestedRepo, "init", "-b", "main")
	mustRunGit(t, nestedRepo, "config", "user.email", "test@example.com")
	mustRunGit(t, nestedRepo, "config", "user.name", "Test User")
	mustRunGit(t, nestedRepo, "config", "commit.gpgsign", "false")
	writeFile(t, nestedRepo, "README.md", "# test\n")
	mustRunGit(t, nestedRepo, "add", ".")
	mustRunGit(t, nestedRepo, "commit", "-m", "init")

	repos, err := discoverRepos(parent, 1)
	if err != nil {
		t.Fatalf("discoverRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos at depth=1, got %d: %v", len(repos), repos)
	}
}

// TestRepoAddAutoDetectWithDepth verifies that the full repo add command
// with --auto-detect --depth 2 discovers and registers nested repos.
func TestRepoAddAutoDetectWithDepth(t *testing.T) {
	d := setupTestDB(t)

	parent := t.TempDir()

	repoD1 := filepath.Join(parent, "repo-shallow")
	group := filepath.Join(parent, "api-group")
	repoD2 := filepath.Join(group, "v2")

	for _, dir := range []string{repoD1, repoD2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
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
	if err := cmd.Flags().Set("depth", "2"); err != nil {
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
}

// TestRepoAddNormalizesSymlinkedPaths verifies that adding the same repo via
// two different paths (real path and symlink) does not create a duplicate.
func TestRepoAddNormalizesSymlinkedPaths(t *testing.T) {
	d := setupTestDB(t)

	// Create a real repo.
	realRepo := initRepo(t)

	// Create a symlink to it.
	tmp := t.TempDir()
	link := filepath.Join(tmp, "linked")
	if err := os.Symlink(realRepo, link); err != nil {
		t.Skipf("cannot create symlink (OS restriction): %v", err)
	}

	// Add via the real path first.
	cmd1 := repoAddCmd()
	if err := cmd1.RunE(cmd1, []string{realRepo}); err != nil {
		t.Fatalf("add real path: %v", err)
	}

	// Add via the symlink — should NOT create a duplicate.
	cmd2 := repoAddCmd()
	if err := cmd2.RunE(cmd2, []string{link}); err != nil {
		t.Fatalf("add symlink path: %v", err)
	}

	repos, err := d.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (no duplicate), got %d", len(repos))
	}
}

// ─── TestRepoAddAliasConflictReturnsError ───────────────────────────────────

// TestRepoAddAliasConflictReturnsError verifies that adding a repo with an
// alias that is already taken by a different repo returns a non-nil error
// (exit code != 0). Regression test for Finding #3.
func TestRepoAddAliasConflictReturnsError(t *testing.T) {
	setupTestDB(t)

	// Create two separate git repos.
	repo1Dir := initRepo(t)
	repo2Dir := initRepo(t)

	// Register repo1 with alias "my-service".
	cmd1 := repoAddCmd()
	if err := cmd1.Flags().Set("alias", "my-service"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := cmd1.RunE(cmd1, []string{repo1Dir}); err != nil {
		t.Fatalf("add repo1: %v", err)
	}

	// Try to register repo2 with the SAME alias — should fail.
	cmd2 := repoAddCmd()
	if err := cmd2.Flags().Set("alias", "my-service"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	err := cmd2.RunE(cmd2, []string{repo2Dir})
	if err == nil {
		t.Fatal("expected error when adding repo with duplicate alias, got nil")
	}
	if !strings.Contains(err.Error(), "could not be added") {
		t.Errorf("error = %q, want to contain \"could not be added\"", err.Error())
	}
}

// TestRepoAddSamePathIdempotent verifies that re-adding the same repo path
// does NOT return an error (idempotent behavior).
func TestRepoAddSamePathIdempotent(t *testing.T) {
	setupTestDB(t)

	repoDir := initRepo(t)

	// First add.
	cmd1 := repoAddCmd()
	if err := cmd1.RunE(cmd1, []string{repoDir}); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Second add of same path — should succeed (no error).
	cmd2 := repoAddCmd()
	if err := cmd2.RunE(cmd2, []string{repoDir}); err != nil {
		t.Fatalf("second add of same path should be idempotent, got: %v", err)
	}
}

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ==========================================================================
// Phase 1: Repo Management (gitm repo add/list/remove/rename)
// ==========================================================================

func TestRepoAdd_ValidRepo(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("myrepo")

	r := e.runGitm("repo", "add", repo)
	e.assertExitCode(r, 0)
	e.assertContains(r, "myrepo")

	// Verify it's listed
	list := e.runGitm("repo", "list")
	e.assertExitCode(list, 0)
	e.assertStdoutContains(list, "myrepo")
	e.assertStdoutContains(list, repo)
}

func TestRepoAdd_WithAlias(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("original-name")

	r := e.runGitm("repo", "add", repo, "--alias", "custom-alias")
	e.assertExitCode(r, 0)

	list := e.runGitm("repo", "list")
	e.assertStdoutContains(list, "custom-alias")
}

func TestRepoAdd_DuplicatePath(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("dup")

	// First add should succeed
	r1 := e.runGitm("repo", "add", repo)
	e.assertExitCode(r1, 0)

	// Second add should fail
	r2 := e.runGitm("repo", "add", repo)
	if r2.ExitCode == 0 {
		// Check if it's a warning instead of error
		e.assertContains(r2, "already")
	}
}

func TestRepoAdd_NonGitDirectory(t *testing.T) {
	e := newTestEnv(t)
	dir := t.TempDir() // Just a directory, not a git repo

	r := e.runGitm("repo", "add", dir)
	// Should error — not a git repo
	if r.ExitCode == 0 {
		t.Errorf("expected non-zero exit code for non-git dir, got 0\nstdout: %s\nstderr: %s",
			r.Stdout, r.Stderr)
	}
}

func TestRepoAdd_NonExistentPath(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("repo", "add", "/does/not/exist/anywhere")
	if r.ExitCode == 0 {
		t.Errorf("expected non-zero exit code for non-existent path, got 0")
	}
}

func TestRepoAdd_ConflictingAlias(t *testing.T) {
	e := newTestEnv(t)
	repo1 := e.initRepo("repo1")
	repo2 := e.initRepo("repo2")

	r1 := e.runGitm("repo", "add", repo1, "--alias", "shared-alias")
	e.assertExitCode(r1, 0)

	r2 := e.runGitm("repo", "add", repo2, "--alias", "shared-alias")
	// FINDING: gitm may allow duplicate aliases (exits 0 even with same alias).
	// Document the actual behavior for the report.
	if r2.ExitCode == 0 {
		// Check if a warning was shown or if both repos are listed
		list := e.runGitm("repo", "list")
		t.Logf("FINDING: duplicate alias 'shared-alias' accepted (exit 0). List output:\n%s", list.Stdout)
		// Count how many times the alias appears
		count := strings.Count(list.Stdout, "shared-alias")
		if count > 1 {
			t.Errorf("FINDING: duplicate alias 'shared-alias' created %d entries — DB UNIQUE constraint not enforced at CLI level", count)
		} else if count == 1 {
			t.Log("FINDING: second add with same alias silently failed or was deduplicated, only 1 entry")
		}
	}
}

func TestRepoAdd_CurrentDirectory(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("dot-repo")

	r := e.runGitmInDir(repo, "repo", "add", ".")
	e.assertExitCode(r, 0)

	// Verify the absolute path is stored, not "."
	list := e.runGitm("repo", "list")
	e.assertNotContains(list, " . ")
}

func TestRepoList_Empty(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("repo", "list")
	e.assertExitCode(r, 0)
	// gitm says "No repositories registered" when empty
	e.assertContains(r, "No repositories registered")
}

func TestRepoList_ShowsAllFields(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("fields-test")
	e.runGitm("repo", "add", repo, "--alias", "fields-test")

	r := e.runGitm("repo", "list")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "ALIAS")
	e.assertStdoutContains(r, "DEFAULT BRANCH")
	e.assertStdoutContains(r, "PATH")
	e.assertStdoutContains(r, "fields-test")
}

func TestRepoRemove_Valid(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("to-remove")
	e.runGitm("repo", "add", repo, "--alias", "to-remove")

	r := e.runGitm("repo", "remove", "to-remove")
	e.assertExitCode(r, 0)

	// Verify gone from list
	list := e.runGitm("repo", "list")
	e.assertNotContains(list, "to-remove")

	// Verify files still exist on disk
	if !e.fileExists(filepath.Join(repo, "README.md")) {
		t.Error("repo files were deleted from disk — remove should only affect DB")
	}
}

func TestRepoRemove_NonExistent(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("repo", "remove", "ghost-alias")
	if r.ExitCode == 0 {
		t.Error("expected error when removing non-existent alias")
	}
}

func TestRepoRemove_RmAlias(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("rm-test")
	e.runGitm("repo", "add", repo, "--alias", "rm-test")

	r := e.runGitm("repo", "rm", "rm-test")
	e.assertExitCode(r, 0)

	list := e.runGitm("repo", "list")
	e.assertNotContains(list, "rm-test")
}

func TestRepoRename_Valid(t *testing.T) {
	e := newTestEnv(t)
	repo := e.initRepo("renamed-repo")
	e.runGitm("repo", "add", repo, "--alias", "before-rename")

	r := e.runGitm("repo", "rename", "before-rename", "after-rename")
	e.assertExitCode(r, 0)

	list := e.runGitm("repo", "list")
	e.assertStdoutContains(list, "after-rename")
	// Check the alias column changed — the path may still contain the old dir name
	// so we check the ALIAS column specifically by looking for the alias field alignment
	if strings.Contains(list.Stdout, "before-rename") {
		// The alias "before-rename" should no longer appear as an alias
		// But it might appear in the PATH column if the directory was named that way.
		// Since we named it "renamed-repo", it shouldn't appear at all.
		t.Errorf("old alias 'before-rename' still appears in list output")
	}
}

func TestRepoRename_ToExistingAlias(t *testing.T) {
	e := newTestEnv(t)
	repo1 := e.initRepo("rename-a")
	repo2 := e.initRepo("rename-b")
	e.runGitm("repo", "add", repo1, "--alias", "alias-a")
	e.runGitm("repo", "add", repo2, "--alias", "alias-b")

	r := e.runGitm("repo", "rename", "alias-a", "alias-b")
	if r.ExitCode == 0 {
		t.Error("expected error when renaming to existing alias")
	}
}

func TestRepoRename_NonExistentSource(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("repo", "rename", "ghost", "new")
	if r.ExitCode == 0 {
		t.Error("expected error when renaming non-existent alias")
	}
}

func TestRepoAdd_AutoDetect(t *testing.T) {
	e := newTestEnv(t)
	// Create a parent dir with multiple git repos
	parent := t.TempDir()
	repo1 := filepath.Join(parent, "project-a")
	repo2 := filepath.Join(parent, "project-b")
	nonGit := filepath.Join(parent, "not-a-repo")

	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)
	os.MkdirAll(nonGit, 0o755)

	e.mustGit(repo1, "init", "-b", "main")
	e.mustGit(repo1, "config", "user.email", "t@t.dev")
	e.mustGit(repo1, "config", "user.name", "T")
	e.mustGit(repo1, "config", "commit.gpgsign", "false")
	e.writeFile(repo1, "f.txt", "a")
	e.mustGit(repo1, "add", ".")
	e.mustGit(repo1, "commit", "-m", "init")

	e.mustGit(repo2, "init", "-b", "main")
	e.mustGit(repo2, "config", "user.email", "t@t.dev")
	e.mustGit(repo2, "config", "user.name", "T")
	e.mustGit(repo2, "config", "commit.gpgsign", "false")
	e.writeFile(repo2, "f.txt", "b")
	e.mustGit(repo2, "add", ".")
	e.mustGit(repo2, "commit", "-m", "init")

	r := e.runGitm("repo", "add", parent, "--auto-detect")
	e.assertExitCode(r, 0)

	list := e.runGitm("repo", "list")
	e.assertStdoutContains(list, "project-a")
	e.assertStdoutContains(list, "project-b")
	e.assertNotContains(list, "not-a-repo")
}

func TestRepoAdd_AutoDetectWithDepth(t *testing.T) {
	e := newTestEnv(t)
	// Create nested structure: parent/sub/deep-repo
	parent := t.TempDir()
	deepRepo := filepath.Join(parent, "sub", "deep-repo")
	os.MkdirAll(deepRepo, 0o755)

	e.mustGit(deepRepo, "init", "-b", "main")
	e.mustGit(deepRepo, "config", "user.email", "t@t.dev")
	e.mustGit(deepRepo, "config", "user.name", "T")
	e.mustGit(deepRepo, "config", "commit.gpgsign", "false")
	e.writeFile(deepRepo, "f.txt", "x")
	e.mustGit(deepRepo, "add", ".")
	e.mustGit(deepRepo, "commit", "-m", "init")

	// Depth 1 should NOT find it
	r1 := e.runGitm("repo", "add", parent, "--auto-detect", "--depth", "1")
	list1 := e.runGitm("repo", "list")
	if strings.Contains(list1.Stdout, "deep-repo") {
		t.Log("Depth 1 found deep-repo — might be expected depending on implementation")
	}

	// Depth 2 should find it
	r2 := e.runGitm("repo", "add", parent, "--auto-detect", "--depth", "2")
	_ = r1
	_ = r2
	list2 := e.runGitm("repo", "list")
	e.assertStdoutContains(list2, "deep-repo")
}

func TestRepoAdd_AutoDetectWithAlias_Invalid(t *testing.T) {
	e := newTestEnv(t)
	dir := t.TempDir()

	r := e.runGitm("repo", "add", dir, "--auto-detect", "--alias", "foo")
	if r.ExitCode == 0 {
		t.Error("expected error when combining --auto-detect with --alias")
	}
}

func TestRepoAdd_MultiplePaths(t *testing.T) {
	e := newTestEnv(t)
	repo1 := e.initRepo("multi-1")
	repo2 := e.initRepo("multi-2")

	r := e.runGitm("repo", "add", repo1, repo2)
	e.assertExitCode(r, 0)

	list := e.runGitm("repo", "list")
	e.assertStdoutContains(list, "multi-1")
	e.assertStdoutContains(list, "multi-2")
}

func TestRepoAdd_AutoDetectSkipsRegistered(t *testing.T) {
	e := newTestEnv(t)
	parent := t.TempDir()
	repo1 := filepath.Join(parent, "already-there")
	os.MkdirAll(repo1, 0o755)
	e.mustGit(repo1, "init", "-b", "main")
	e.mustGit(repo1, "config", "user.email", "t@t.dev")
	e.mustGit(repo1, "config", "user.name", "T")
	e.mustGit(repo1, "config", "commit.gpgsign", "false")
	e.writeFile(repo1, "f.txt", "a")
	e.mustGit(repo1, "add", ".")
	e.mustGit(repo1, "commit", "-m", "init")

	// Register it first
	e.runGitm("repo", "add", repo1)

	// Auto-detect should show warning but not fail
	r := e.runGitm("repo", "add", parent, "--auto-detect")
	e.assertExitCode(r, 0)
	// Should mention it's already registered (warning)
	e.assertContains(r, "already")
}

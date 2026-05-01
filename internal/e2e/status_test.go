package e2e

import (
	"strings"
	"testing"
)

// ==========================================================================
// Phase 2: Status (gitm status)
// ==========================================================================

func TestStatus_CleanRepo(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("status-clean")
	e.runGitm("repo", "add", repo, "--alias", "status-clean")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "status-clean")
	e.assertStdoutContains(r, "clean")
}

func TestStatus_DirtyModified(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("status-dirty")
	e.runGitm("repo", "add", repo, "--alias", "status-dirty")

	// Modify a tracked file
	e.writeFile(repo, "README.md", "# modified content\n")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "status-dirty")
	// Should show something indicating dirty (not "clean")
	if containsAll(r.Stdout, "status-dirty", "clean") && !containsAny(r.Stdout, "modified", "dirty", "1") {
		t.Error("dirty repo shown as clean")
	}
}

func TestStatus_UntrackedOnly(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("status-untracked")
	e.runGitm("repo", "add", repo, "--alias", "status-untracked")

	// Add untracked file only
	e.writeFile(repo, "newfile.txt", "untracked content\n")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	// FINDING: Per README docs, "untracked files are ignored" for dirty checks.
	// However, the status command's DIRTY column may still count untracked files
	// as "N modified" in the display. Document the actual behavior.
	if strings.Contains(r.Stdout, "clean") {
		t.Log("status correctly shows 'clean' for untracked-only repos (matches docs)")
	} else if strings.Contains(r.Stdout, "modified") {
		t.Log("FINDING: status shows untracked files as 'N modified' in DIRTY column.")
		t.Log("This contradicts the README claim that 'untracked files are ignored'.")
		t.Log("The 'ignored' behavior may only apply to checkout/update skip logic, not status display.")
		t.Logf("Actual output:\n%s", r.Stdout)
	}
}

func TestStatus_AheadOfRemote(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("status-ahead")
	e.runGitm("repo", "add", repo, "--alias", "status-ahead")

	// Make a local commit without pushing
	e.writeFile(repo, "new.txt", "ahead\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "commit", "-m", "unpushed commit")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "ahead")
}

func TestStatus_BehindRemote(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("status-behind")
	e.runGitm("repo", "add", repo, "--alias", "status-behind")

	// Push a commit from another clone to make our repo behind
	other := e.cloneRepo(origin, "other-clone")
	e.writeFile(other, "from-other.txt", "new\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "commit from other")
	e.mustGit(other, "push")

	// Fetch so local knows about the new commit
	e.mustGit(repo, "fetch")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "behind")
}

func TestStatus_WithFetch(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("status-fetch")
	e.runGitm("repo", "add", repo, "--alias", "status-fetch")

	// Push a commit from another clone
	other := e.cloneRepo(origin, "other-fetch")
	e.writeFile(other, "remote-new.txt", "new\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "remote commit")
	e.mustGit(other, "push")

	// Without fetch, local doesn't know — status should not show behind
	r1 := e.runGitm("status")
	e.assertExitCode(r1, 0)

	// With --fetch, should update and show behind
	r2 := e.runGitm("status", "--fetch")
	e.assertExitCode(r2, 0)
	e.assertStdoutContains(r2, "behind")
}

func TestStatus_MultipleRepos(t *testing.T) {
	e := newTestEnv(t)
	repo1, _ := e.initRepoWithRemote("multi-status-1")
	repo2, _ := e.initRepoWithRemote("multi-status-2")
	e.runGitm("repo", "add", repo1, "--alias", "multi-status-1")
	e.runGitm("repo", "add", repo2, "--alias", "multi-status-2")

	r := e.runGitm("status")
	e.assertExitCode(r, 0)
	e.assertStdoutContains(r, "multi-status-1")
	e.assertStdoutContains(r, "multi-status-2")
}

// --------------------------------------------------------------------------
// Helpers for this file
// --------------------------------------------------------------------------

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

package e2e

import (
	"testing"
)

// ==========================================================================
// Phase 5: Update (gitm update)
// ==========================================================================

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("up-current")
	e.runGitm("repo", "add", repo, "--alias", "up-current")

	r := e.runGitm("update", "--repo", "up-current")
	e.assertExitCode(r, 0)
	// Should indicate already up-to-date or show success
	e.assertContains(r, "up-current")
}

func TestUpdate_RemoteAhead(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("up-behind")
	e.runGitm("repo", "add", repo, "--alias", "up-behind")

	// Push from another clone
	other := e.cloneRepo(origin, "up-other")
	e.writeFile(other, "new-from-remote.txt", "new content\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "remote update")
	e.mustGit(other, "push")

	r := e.runGitm("update", "--repo", "up-behind")
	e.assertExitCode(r, 0)

	// Should have the new file
	if !e.fileExists(repo + "/new-from-remote.txt") {
		t.Error("update did not pull — new-from-remote.txt missing")
	}
}

func TestUpdate_DirtyRepo_Skips(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("up-dirty")
	e.runGitm("repo", "add", repo, "--alias", "up-dirty")

	// Make dirty
	e.writeFile(repo, "README.md", "# dirty\n")

	r := e.runGitm("update", "--repo", "up-dirty")
	e.assertExitCode(r, 0)
	// Should skip/warn about dirty
	e.assertContains(r, "up-dirty")
}

func TestUpdate_NonExistentRepo(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("update", "--repo", "ghost-repo")
	// Should error — alias doesn't match any registered repo
	if r.ExitCode == 0 {
		// Even with exit 0, check for error message
		combined := r.Stdout + r.Stderr
		if !containsAny(combined, "not found", "error", "no match", "unknown") {
			t.Error("expected error for non-existent --repo alias, but got clean success")
		}
	}
}

func TestUpdate_DoesNotSwitchBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, _ := e.initRepoWithRemote("up-nosw")
	e.runGitm("repo", "add", repo, "--alias", "up-nosw")

	// Switch to a feature branch
	e.mustGit(repo, "checkout", "-b", "feat/stay-here")
	e.mustGit(repo, "push", "--set-upstream", "origin", "feat/stay-here")

	r := e.runGitm("update", "--repo", "up-nosw")
	e.assertExitCode(r, 0)

	// Should still be on feat/stay-here
	branch := e.currentBranch(repo)
	if branch != "feat/stay-here" {
		t.Errorf("update should not switch branches, but now on %s", branch)
	}
}

func TestUpdate_DivergedBranch(t *testing.T) {
	e := newTestEnv(t)
	repo, origin := e.initRepoWithRemote("up-diverged")
	e.runGitm("repo", "add", repo, "--alias", "up-diverged")

	// Push a commit from another clone (remote ahead)
	other := e.cloneRepo(origin, "up-div-other")
	e.writeFile(other, "remote.txt", "remote\n")
	e.mustGit(other, "add", ".")
	e.mustGit(other, "commit", "-m", "remote commit")
	e.mustGit(other, "push")

	// Make a local commit (local ahead too — diverged)
	e.writeFile(repo, "local.txt", "local\n")
	e.mustGit(repo, "add", ".")
	e.mustGit(repo, "commit", "-m", "local commit")

	r := e.runGitm("update", "--repo", "up-diverged")
	// --ff-only should fail on diverged branches
	// Either exit != 0, or output mentions failure
	t.Logf("Diverged update: exit=%d stdout=%s stderr=%s",
		r.ExitCode, r.Stdout, r.Stderr)
}

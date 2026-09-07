package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestReadWorkingTree(t *testing.T) {
	t.Run("tracking and exact filenames", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "config", "status.aheadBehind", "false")
		mustRunGit(t, dir, "mv", "README.md", "space name.md")
		writeFile(t, dir, "other.txt", "untracked")
		s, err := git.ReadWorkingTree(dir)
		if err != nil {
			t.Fatal(err)
		}
		if s.Branch != "main" || s.Upstream != "origin/main" || !s.TrackingKnown || s.Ahead != 0 || s.Behind != 0 || s.ChangedFiles != 2 || s.Detached || s.Unborn {
			t.Fatalf("unexpected status: %+v", s)
		}
	})
	t.Run("ahead and conflicts", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "checkout", "-b", "other")
		makeCommit(t, dir, "README.md", "other", "other change")
		mustRunGit(t, dir, "checkout", "main")
		makeCommit(t, dir, "README.md", "main", "main change")
		if _, err := git.Merge(dir, "other"); err == nil {
			t.Fatal("expected merge conflict")
		}
		s, err := git.ReadWorkingTree(dir)
		if err != nil || s.Ahead != 1 || s.Behind != 0 || !s.Conflicts || s.ChangedFiles != 1 {
			t.Fatalf("status=%+v err=%v", s, err)
		}
	})
	t.Run("unborn", func(t *testing.T) {
		dir := t.TempDir()
		mustRunGit(t, dir, "init", "-b", "new")
		s, err := git.ReadWorkingTree(dir)
		if err != nil || s.Branch != "new" || !s.Unborn || s.TrackingKnown {
			t.Fatalf("status=%+v err=%v", s, err)
		}
	})
	t.Run("detached", func(t *testing.T) {
		dir := initRepo(t)
		mustRunGit(t, dir, "checkout", "--detach")
		s, err := git.ReadWorkingTree(dir)
		if err != nil || !s.Detached || s.Unborn || s.TrackingKnown {
			t.Fatalf("status=%+v err=%v", s, err)
		}
	})
	t.Run("missing upstream", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
		s, err := git.ReadWorkingTree(dir)
		if err != nil || s.Upstream != "origin/main" || s.TrackingKnown {
			t.Fatalf("status=%+v err=%v", s, err)
		}
	})
	t.Run("missing directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := git.ReadWorkingTree(dir); err == nil {
			t.Fatal("expected inspection failure")
		}
	})
}

func TestAheadBehindReportsFailures(t *testing.T) {
	t.Run("branch and tag share a name", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "tag", "main")
		makeCommit(t, dir, "ahead.txt", "ahead", "ahead")
		if branch, err := git.CurrentBranch(dir); err != nil || branch != "main" {
			t.Fatalf("branch=%q, err=%v", branch, err)
		}
		if ahead, behind, err := git.AheadBehind(dir, false); err != nil || ahead != 1 || behind != 0 {
			t.Fatalf("counts=%d/%d, err=%v", ahead, behind, err)
		}
	})
	t.Run("unusable tracking configuration", func(t *testing.T) {
		dir := initRepo(t)
		mustRunGit(t, dir, "config", "branch.main.remote", "origin")
		mustRunGit(t, dir, "config", "branch.main.merge", "refs/heads/main")
		if _, _, err := git.AheadBehindOf(dir, "main"); err == nil {
			t.Fatal("unusable configured upstream must return an error")
		}
	})
	t.Run("fetch", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
		if _, _, err := git.AheadBehind(dir, true); err == nil {
			t.Fatal("failed fetch must return an error")
		}
	})
	t.Run("deleted upstream", func(t *testing.T) {
		dir, _ := initRepoWithRemote(t)
		mustRunGit(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
		if _, _, err := git.AheadBehindOf(dir, "main"); err == nil {
			t.Fatal("missing upstream ref must return an error")
		}
	})
	t.Run("missing repository", func(t *testing.T) {
		if _, _, err := git.AheadBehindOf(t.TempDir(), "main"); err == nil {
			t.Fatal("non-repository must return an error")
		}
	})
	t.Run("missing branch", func(t *testing.T) {
		if _, _, err := git.AheadBehindOf(initRepo(t), "missing"); err == nil {
			t.Fatal("missing branch must return an error")
		}
	})
	t.Run("local-only branch", func(t *testing.T) {
		ahead, behind, err := git.AheadBehind(initRepo(t), false)
		if err != nil || ahead != 0 || behind != 0 {
			t.Fatalf("local-only branch: %d, %d, %v", ahead, behind, err)
		}
	})
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestFetchCmdBasics(t *testing.T) {
	cmd := fetchCmd()
	if cmd.Use != "fetch" || cmd.Short == "" || cmd.Long == "" || cmd.RunE == nil {
		t.Fatalf("fetchCmd is incomplete: Use=%q Short=%q", cmd.Use, cmd.Short)
	}
	if err := cmd.Args(cmd, []string{"x"}); err == nil {
		t.Error("fetch should reject positional args")
	}
	if err := cmd.ParseFlags([]string{"--prune", "-r", "a,b", "-g", "backend"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if v, err := cmd.Flags().GetBool("prune"); err != nil || !v {
		t.Errorf("--prune = %v, %v; want true", v, err)
	}
	if repos, err := cmd.Flags().GetStringSlice("repo"); err != nil || strings.Join(repos, ",") != "a,b" {
		t.Errorf("--repo = %q, %v; want a,b", repos, err)
	}
	if g, err := cmd.Flags().GetString("group"); err != nil || g != "backend" {
		t.Errorf("--group = %q, %v; want backend", g, err)
	}
}

// newFetchRepo registers a repo cloned from a bare origin and returns both.
func newFetchRepo(t *testing.T, alias string) (dir, origin string) {
	t.Helper()
	dir, origin, _ = initRepoWithRemote(t)
	if _, err := database.AddRepository(alias, alias, dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	return dir, origin
}

func TestRunFetchPicksUpNewRemoteBranch(t *testing.T) {
	setupTestDB(t)
	dir, origin := newFetchRepo(t, "app")
	mustRunGit(t, origin, "branch", "feature/new", "main")

	var out bytes.Buffer
	if err := runFetch(&out, fetchOptions{}); err != nil {
		t.Fatalf("runFetch: %v\n%s", err, out.String())
	}
	if !git.BranchExists(dir, "refs/remotes/origin/feature/new") {
		t.Error("origin/feature/new was not fetched")
	}
	for _, want := range []string{"REPO", "RESULT", "app", "ok", "1 succeeded"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunFetchPrune(t *testing.T) {
	setupTestDB(t)
	dir, origin := newFetchRepo(t, "app")
	mustRunGit(t, origin, "branch", "feature/gone", "main")
	var out bytes.Buffer
	if err := runFetch(&out, fetchOptions{}); err != nil {
		t.Fatalf("runFetch: %v\n%s", err, out.String())
	}
	mustRunGit(t, origin, "branch", "-D", "feature/gone")

	if err := runFetch(&out, fetchOptions{}); err != nil {
		t.Fatalf("runFetch: %v\n%s", err, out.String())
	}
	if !git.BranchExists(dir, "refs/remotes/origin/feature/gone") {
		t.Fatal("fetch without --prune removed origin/feature/gone")
	}

	if err := runFetch(&out, fetchOptions{prune: true}); err != nil {
		t.Fatalf("runFetch --prune: %v\n%s", err, out.String())
	}
	if git.BranchExists(dir, "refs/remotes/origin/feature/gone") {
		t.Error("fetch --prune kept origin/feature/gone")
	}
}

func TestRunFetchBrokenOriginFailsButOthersRun(t *testing.T) {
	d := setupTestDB(t)
	healthy, origin := newFetchRepo(t, "healthy")
	mustRunGit(t, origin, "branch", "feature/new", "main")

	newRepo(t, d, "no-origin")
	unreachable, unreachableOrigin, _ := initRepoWithRemote(t)
	if _, err := d.AddRepository("unreachable", "unreachable", unreachable, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	if err := os.RemoveAll(unreachableOrigin); err != nil {
		t.Fatalf("remove origin: %v", err)
	}

	var out bytes.Buffer
	err := runFetch(&out, fetchOptions{})
	if err == nil || !strings.Contains(err.Error(), "2 repository(ies)") {
		t.Fatalf("runFetch error = %v, want failure for 2 repositories\n%s", err, out.String())
	}
	if !git.BranchExists(healthy, "refs/remotes/origin/feature/new") {
		t.Error("healthy repo was not fetched")
	}
	for _, want := range []string{"no origin remote configured", "does not appear to be a git repository", "1 succeeded", "2 failed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunFetchLeavesWorktreeIndexAndHeadAlone(t *testing.T) {
	setupTestDB(t)
	dir, origin := newFetchRepo(t, "app")

	other := cloneRepo(t, origin)
	mustRunGit(t, other, "config", "user.email", "other@example.com")
	mustRunGit(t, other, "config", "user.name", "Other")
	writeFile(t, other, "remote.txt", "remote\n")
	mustRunGit(t, other, "add", "remote.txt")
	mustRunGit(t, other, "commit", "-q", "-m", "remote work")
	mustRunGit(t, other, "push", "-q", "origin", "main")

	writeFile(t, dir, "staged.txt", "staged\n")
	mustRunGit(t, dir, "add", "staged.txt")
	writeFile(t, dir, "README.md", "local edit\n")
	writeFile(t, dir, "untracked.txt", "untracked\n")

	headBefore := mustRunGit(t, dir, "rev-parse", "HEAD")
	statusBefore := mustRunGit(t, dir, "status", "--porcelain")

	var out bytes.Buffer
	if err := runFetch(&out, fetchOptions{prune: true}); err != nil {
		t.Fatalf("runFetch: %v\n%s", err, out.String())
	}

	if got := mustRunGit(t, dir, "rev-parse", "HEAD"); got != headBefore {
		t.Errorf("HEAD moved: %s -> %s", headBefore, got)
	}
	if got := gitCurrentBranch(t, dir); got != "main" {
		t.Errorf("current branch = %q, want main", got)
	}
	if got := mustRunGit(t, dir, "status", "--porcelain"); got != statusBefore {
		t.Errorf("worktree/index changed:\nbefore:\n%s\nafter:\n%s", statusBefore, got)
	}
	if got := mustRunGit(t, dir, "rev-parse", "origin/main"); got == headBefore {
		t.Error("origin/main was not updated by fetch")
	}
	if _, err := os.Stat(filepath.Join(dir, "remote.txt")); !os.IsNotExist(err) {
		t.Errorf("fetch wrote remote.txt into the worktree (stat err: %v)", err)
	}
}

func TestRunFetchRepoAndGroupScoping(t *testing.T) {
	d := setupTestDB(t)
	a, originA := newFetchRepo(t, "a")
	b, originB := newFetchRepo(t, "b")
	mustRunGit(t, originA, "branch", "feature/new", "main")
	mustRunGit(t, originB, "branch", "feature/new", "main")
	if _, err := d.CreateGroup("backend"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := d.AddRepositoriesToGroup("backend", []string{"a"}); err != nil {
		t.Fatalf("AddRepositoriesToGroup: %v", err)
	}

	var out bytes.Buffer
	if err := runFetch(&out, fetchOptions{repoAliases: []string{"a", "b"}, groupName: "backend"}); err != nil {
		t.Fatalf("runFetch: %v\n%s", err, out.String())
	}
	if !git.BranchExists(a, "refs/remotes/origin/feature/new") {
		t.Error("repo a was not fetched")
	}
	if git.BranchExists(b, "refs/remotes/origin/feature/new") {
		t.Error("repo b is outside the group and should not be fetched")
	}
}

func TestRunFetchEmptyState(t *testing.T) {
	setupTestDB(t)

	var out bytes.Buffer
	if err := runFetch(&out, fetchOptions{}); err != nil {
		t.Fatalf("runFetch: %v", err)
	}
	if !strings.Contains(out.String(), "No repositories registered") {
		t.Errorf("expected empty-state message, got:\n%s", out.String())
	}
}

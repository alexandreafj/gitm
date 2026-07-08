package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestRunReset_InvalidCommits(t *testing.T) {
	if err := runReset(resetModeMixed, 0, nil); err == nil {
		t.Fatal("expected error for commits < 1")
	}
}

func TestRunReset_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	if err := runReset(resetModeMixed, 1, nil); err != nil {
		t.Fatalf("runReset: %v", err)
	}
}

func TestRunReset_NoEnoughCommits(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runReset(resetModeMixed, 3, nil); err != nil {
		t.Fatalf("runReset: %v", err)
	}
}

func TestRunReset_Mixed(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	writeFile(t, repoDir, "a.txt", "a\n")
	mustRunGit(t, repoDir, "add", "a.txt")
	mustRunGit(t, repoDir, "commit", "-m", "commit a")
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	repo, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	ui := fakeUI{selectRepos: []*db.Repository{repo}}
	if err := runResetWithUI(ui, resetModeMixed, 1, nil); err != nil {
		t.Fatalf("runResetWithUI: %v", err)
	}
}

func TestRunReset_DryRunDoesNotMoveHead(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	writeFile(t, repoDir, "a.txt", "a\n")
	mustRunGit(t, repoDir, "add", "a.txt")
	mustRunGit(t, repoDir, "commit", "-m", "commit a")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	before := mustRunGit(t, repoDir, "rev-parse", "HEAD")

	repo, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	ui := fakeUI{selectRepos: []*db.Repository{repo}}
	output := captureOutput(t, func() {
		if err := runResetWithUIDryRun(ui, resetModeMixed, 1, nil, true); err != nil {
			t.Fatalf("runResetWithUIDryRun: %v", err)
		}
	})

	after := mustRunGit(t, repoDir, "rev-parse", "HEAD")
	if after != before {
		t.Fatalf("HEAD moved during dry-run: before %s after %s", before, after)
	}
	log := mustRunGit(t, repoDir, "log", "--oneline")
	if !strings.Contains(log, "commit a") {
		t.Fatal("commit a should still exist after dry-run")
	}
	if !strings.Contains(output, "git reset --mixed HEAD~1") {
		t.Fatalf("expected reset command preview, got:\n%s", output)
	}
}

func TestRunReset_SelectionError(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	writeFile(t, repoDir, "a.txt", "a\n")
	mustRunGit(t, repoDir, "add", "a.txt")
	mustRunGit(t, repoDir, "commit", "-m", "commit a")
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	ui := fakeUI{selectErr: errors.New("canceled")}
	if err := runResetWithUI(ui, resetModeMixed, 1, nil); err == nil {
		t.Fatal("expected error from selection")
	}
}

func TestOfferForcePush_SkipOnNoInput(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	f, err := os.CreateTemp("", "gitm-input")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString("n\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	os.Stdin = f

	offerForcePush([]*db.Repository{{Alias: "repo1", Path: "/tmp/repo"}}, 1)
}

func TestRunReset_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir := initRepo(t)
	writeFile(t, repo1Dir, "a.txt", "a\n")
	mustRunGit(t, repo1Dir, "add", "a.txt")
	mustRunGit(t, repo1Dir, "commit", "-m", "commit a")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir := initRepo(t)
	writeFile(t, repo2Dir, "b.txt", "b\n")
	mustRunGit(t, repo2Dir, "add", "b.txt")
	mustRunGit(t, repo2Dir, "commit", "-m", "commit b")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	if err := runResetWithUI(fakeUI{}, resetModeMixed, 1, []string{"repo1"}); err != nil {
		t.Fatalf("runResetWithUI: %v", err)
	}

	log1 := mustRunGit(t, repo1Dir, "log", "--oneline")
	if strings.Contains(log1, "commit a") {
		t.Error("repo1: commit a should have been reset")
	}

	log2 := mustRunGit(t, repo2Dir, "log", "--oneline")
	if !strings.Contains(log2, "commit b") {
		t.Error("repo2: commit b should still exist (not in --repo list)")
	}
}

// Enough repos that runner.Run's goroutines genuinely overlap: this test
// exists to fail under -race if the reset closure ever shares state with the
// enclosing function again.
func TestRunReset_ManyReposParallel(t *testing.T) {
	database = setupTestDB(t)

	dirs := make([]string, 8)
	for i := range dirs {
		_, dir := newRepo(t, database, fmt.Sprintf("repo%d", i))
		writeFile(t, dir, "a.txt", "a\n")
		mustRunGit(t, dir, "add", "a.txt")
		mustRunGit(t, dir, "commit", "-m", "commit to reset")
		dirs[i] = dir
	}

	if err := runResetWithUI(fakeUI{}, resetModeMixed, 1, nil); err != nil {
		t.Fatalf("runResetWithUI: %v", err)
	}

	for i, dir := range dirs {
		log := mustRunGit(t, dir, "log", "--oneline")
		if strings.Contains(log, "commit to reset") {
			t.Errorf("repo%d: commit should have been reset", i)
		}
	}
}

func TestGatherResetInfo_NoUpstreamNothingPushed(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	mustRunGit(t, dir, "add", "a.txt")
	mustRunGit(t, dir, "commit", "-m", "local commit")
	repo := &db.Repository{Alias: "local-only", Path: dir, DefaultBranch: "main"}

	infos, skipped := gatherResetInfo([]*db.Repository{repo}, 1, "HEAD~1")
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].pushedCount != 0 {
		t.Errorf("branch has no upstream, so nothing can be pushed; pushedCount = %d, want 0", infos[0].pushedCount)
	}
}

func TestGatherResetInfo_PushedCommitCounted(t *testing.T) {
	repoDir, _, branch := initRepoWithRemote(t)
	writeFile(t, repoDir, "a.txt", "a\n")
	mustRunGit(t, repoDir, "add", "a.txt")
	mustRunGit(t, repoDir, "commit", "-m", "pushed commit")
	mustRunGit(t, repoDir, "push", "origin", branch)
	repo := &db.Repository{Alias: "with-remote", Path: repoDir, DefaultBranch: branch}

	infos, skipped := gatherResetInfo([]*db.Repository{repo}, 1, "HEAD~1")
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].pushedCount != 1 {
		t.Errorf("commit is on origin; pushedCount = %d, want 1", infos[0].pushedCount)
	}
}

func TestRunReset_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runResetWithUI(fakeUI{}, resetModeMixed, 1, []string{"ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-repo") {
		t.Errorf("error should mention ghost-repo, got: %v", err)
	}
}

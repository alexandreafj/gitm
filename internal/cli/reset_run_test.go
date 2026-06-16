package cli

import (
	"errors"
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

package cli

import (
	"errors"
	"os"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

func TestRunReset_InvalidCommits(t *testing.T) {
	if err := runReset(resetModeMixed, 0); err == nil {
		t.Fatal("expected error for commits < 1")
	}
}

func TestRunReset_NoRepos(t *testing.T) {
	database = setupTestDB(t)
	if err := runReset(resetModeMixed, 1); err != nil {
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

	if err := runReset(resetModeMixed, 3); err != nil {
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
	if err := runResetWithUI(ui, resetModeMixed, 1); err != nil {
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
	if err := runResetWithUI(ui, resetModeMixed, 1); err == nil {
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

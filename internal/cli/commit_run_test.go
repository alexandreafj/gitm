package cli

import (
	"errors"
	"testing"
)

func TestRunCommit_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runCommit(false); err == nil {
		t.Fatal("expected error when no repositories registered")
	}
}

func TestRunCommit_NoDirtyRepos(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runCommit(false); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
}

func TestRunCommit_CanceledSelection(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	ui := fakeUI{selectErr: errors.New("canceled")}
	if err := runCommitWithUI(ui, false); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestRunCommit_SuccessNoPush(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	ui := fakeUI{}
	if err := runCommitWithUI(ui, true); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestRunCommit_FileSelectError(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	ui := fakeUI{fileErr: errors.New("boom")}
	if err := runCommitWithUI(ui, true); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestRunCommit_CommitMessageCanceled(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	ui := fakeUI{commitErr: errors.New("canceled")}
	if err := runCommitWithUI(ui, true); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestRunCommit_StageError(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	ui := fakeUI{fileSelect: []string{"?? missing.txt"}}
	if err := runCommitWithUI(ui, true); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("one\ntwo"); got != "one" {
		t.Fatalf("firstLine = %q", got)
	}
	if got := firstLine("single"); got != "single" {
		t.Fatalf("firstLine = %q", got)
	}
}

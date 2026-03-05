package cli

import (
	"testing"
)

func TestRunDiscard_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runDiscardWithUI(fakeUI{}); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}
}

func TestRunDiscard_NoDirtyRepos(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runDiscardWithUI(fakeUI{}); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}
}

func TestRunDiscard_DirtyRepos(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	if err := runDiscardWithUI(fakeUI{}); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}
}

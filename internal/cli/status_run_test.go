package cli

import (
	"testing"
)

func TestRunStatus_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runStatus(nil, nil, false); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
}

func TestRunStatus_CleanRepo(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runStatus(nil, nil, false); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
}

func TestRunStatus_DirtyRepo(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	if err := runStatus(nil, nil, false); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
}

func TestPrintStatusTable(t *testing.T) {
	statuses := []repoStatus{
		{name: "repo1", branch: "main", dirty: "clean", ahead: 0, behind: 0},
		{name: "repo2", branch: "feat", dirty: "2 modified", ahead: 1, behind: 0},
		{name: "repo3", err: "boom"},
	}
	printStatusTable(statuses)
}

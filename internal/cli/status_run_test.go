package cli

import (
	"testing"
)

func TestRunStatus_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runStatus(false, nil); err != nil {
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

	if err := runStatus(false, nil); err != nil {
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

	if err := runStatus(false, nil); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
}

func TestRunStatus_RepoFlag_FiltersRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir := initRepo(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	if err := runStatus(false, []string{"repo1"}); err != nil {
		t.Fatalf("runStatus with -r: %v", err)
	}
}

func TestRunStatus_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runStatus(false, []string{"ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
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

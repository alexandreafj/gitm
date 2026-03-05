package cli

import (
	"testing"
)

func TestRunStashPush_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runStashPushWithUI(fakeUI{}); err != nil {
		t.Fatalf("runStashPush: %v", err)
	}
}

func TestRunStashPush_Stashes(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "stash.txt", "stash\n")

	if err := runStashPushWithUI(fakeUI{}); err != nil {
		t.Fatalf("runStashPush: %v", err)
	}
}

func TestRunStashApply_NoReposWithStash(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runStashApplyOrPopWithUI(fakeUI{}, false); err != nil {
		t.Fatalf("runStashApply: %v", err)
	}
}

func TestRunStashApplyAndPop(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "stash.txt", "stash\n")
	mustRunGit(t, repoDir, "add", "stash.txt")
	mustRunGit(t, repoDir, "commit", "-m", "stash base")
	writeFile(t, repoDir, "stash.txt", "stash change\n")
	mustRunGit(t, repoDir, "stash", "push", "-m", "test stash")

	if err := runStashApplyOrPopWithUI(fakeUI{}, false); err != nil {
		t.Fatalf("runStashApply: %v", err)
	}
	if err := runStashApplyOrPopWithUI(fakeUI{}, true); err != nil {
		t.Fatalf("runStashPop: %v", err)
	}
}

func TestRunStashList(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "stash.txt", "stash\n")
	mustRunGit(t, repoDir, "stash", "push", "-m", "test stash")

	if err := runStashList(nil, nil); err != nil {
		t.Fatalf("runStashList: %v", err)
	}
}

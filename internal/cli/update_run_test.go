package cli

import (
	"testing"
)

func TestRunUpdate_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runUpdate(nil, nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

func TestRunUpdate_SkipsDirtyTracked(t *testing.T) {
	database = setupTestDB(t)
	repo, dir := newRepo(t, database, "repo1")

	writeFile(t, dir, "README.md", "changed\n")

	if err := runUpdate(nil, nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	_ = repo
}

func TestRunUpdate_Pulls(t *testing.T) {
	database = setupTestDB(t)
	repo, origin, _ := initRepoWithRemote(t)

	_, err := database.AddRepository("repo1", "repo1", repo, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	clone := cloneRepo(t, origin)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	writeFile(t, clone, "from-remote.txt", "remote\n")
	mustRunGit(t, clone, "add", "from-remote.txt")
	mustRunGit(t, clone, "commit", "-m", "remote change")
	mustRunGit(t, clone, "push")

	if err := runUpdate(nil, nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

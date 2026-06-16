package cli

import (
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestRunStashPush_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runStashPushWithUI(fakeUI{}, nil); err != nil {
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

	if err := runStashPushWithUI(fakeUI{}, nil); err != nil {
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

	if err := runStashApplyOrPopWithUI(fakeUI{}, false, nil); err != nil {
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

	if err := runStashApplyOrPopWithUI(fakeUI{}, false, nil); err != nil {
		t.Fatalf("runStashApply: %v", err)
	}
	if err := runStashApplyOrPopWithUI(fakeUI{}, true, nil); err != nil {
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

	if err := runStashListFn(nil); err != nil {
		t.Fatalf("runStashList: %v", err)
	}
}

func TestRunStashPush_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir := initRepo(t)
	writeFile(t, repo1Dir, "dirty1.txt", "dirty\n")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir := initRepo(t)
	writeFile(t, repo2Dir, "dirty2.txt", "dirty\n")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	if err := runStashPushWithUI(fakeUI{}, []string{"repo1"}); err != nil {
		t.Fatalf("runStashPushWithUI: %v", err)
	}

	has1, err := git.HasStash(repo1Dir)
	if err != nil {
		t.Fatalf("HasStash repo1: %v", err)
	}
	if !has1 {
		t.Error("repo1: expected stash entry after stash push with -r repo1")
	}

	has2, err := git.HasStash(repo2Dir)
	if err != nil {
		t.Fatalf("HasStash repo2: %v", err)
	}
	if has2 {
		t.Error("repo2: should NOT have stash entry (not in --repo list)")
	}
}

func TestRunStashPush_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runStashPushWithUI(fakeUI{}, []string{"repo1", "ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-repo") {
		t.Errorf("error should mention ghost-repo, got: %v", err)
	}
}

func TestRunStashPush_RepoFlag_SkipsCleanRepoSilently(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runStashPushWithUI(fakeUI{}, []string{"repo1"}); err != nil {
		t.Fatalf("runStashPushWithUI: %v", err)
	}

	has, err := git.HasStash(repo1Dir)
	if err != nil {
		t.Fatalf("HasStash: %v", err)
	}
	if has {
		t.Error("repo1: should NOT have stash entry (repo is clean)")
	}
}

func TestRunStashApplyOrPop_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir := initRepo(t)
	writeFile(t, repo1Dir, "f.txt", "a\n")
	mustRunGit(t, repo1Dir, "add", "f.txt")
	mustRunGit(t, repo1Dir, "commit", "-m", "base")
	writeFile(t, repo1Dir, "f.txt", "b\n")
	mustRunGit(t, repo1Dir, "stash", "push", "-m", "stash1")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir := initRepo(t)
	writeFile(t, repo2Dir, "g.txt", "a\n")
	mustRunGit(t, repo2Dir, "add", "g.txt")
	mustRunGit(t, repo2Dir, "commit", "-m", "base")
	writeFile(t, repo2Dir, "g.txt", "b\n")
	mustRunGit(t, repo2Dir, "stash", "push", "-m", "stash2")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	if err := runStashApplyOrPopWithUI(fakeUI{}, false, []string{"repo1"}); err != nil {
		t.Fatalf("runStashApplyOrPopWithUI: %v", err)
	}

	dirty1, err := git.IsDirty(repo1Dir)
	if err != nil {
		t.Fatalf("IsDirty repo1: %v", err)
	}
	if !dirty1 {
		t.Error("repo1: expected dirty after stash apply")
	}

	dirty2, err := git.IsDirty(repo2Dir)
	if err != nil {
		t.Fatalf("IsDirty repo2: %v", err)
	}
	if dirty2 {
		t.Error("repo2: should NOT be dirty (not in --repo list)")
	}
}

func TestRunStashApplyOrPop_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runStashApplyOrPopWithUI(fakeUI{}, false, []string{"ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
}

func TestRunStashList_RepoFlag_FiltersRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir := initRepo(t)
	writeFile(t, repo1Dir, "f.txt", "stash\n")
	mustRunGit(t, repo1Dir, "stash", "push", "-m", "stash1")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir := initRepo(t)
	writeFile(t, repo2Dir, "g.txt", "stash\n")
	mustRunGit(t, repo2Dir, "stash", "push", "-m", "stash2")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	if err := runStashListFn([]string{"repo1"}); err != nil {
		t.Fatalf("runStashListFn: %v", err)
	}
}

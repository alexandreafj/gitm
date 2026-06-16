package git_test

import (
	"os/exec"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestRemoteConfiguredDetectsOrigin(t *testing.T) {
	repo, origin := initRepoWithRemote(t)
	if origin == "" {
		t.Fatal("expected test origin path")
	}

	ok, err := git.RemoteConfigured(repo, "origin")
	if err != nil {
		t.Fatalf("RemoteConfigured: %v", err)
	}
	if !ok {
		t.Fatal("expected origin to be configured")
	}
}

func TestRemoteConfiguredDetectsMissingRemote(t *testing.T) {
	repo := initRepo(t)

	ok, err := git.RemoteConfigured(repo, "origin")
	if err != nil {
		t.Fatalf("RemoteConfigured: %v", err)
	}
	if ok {
		t.Fatal("expected origin to be missing")
	}
}

func TestHasUpstreamDetectsConfiguredUpstream(t *testing.T) {
	repo, _ := initRepoWithRemote(t)

	ok, err := git.HasUpstream(repo)
	if err != nil {
		t.Fatalf("HasUpstream: %v", err)
	}
	if !ok {
		t.Fatal("expected upstream to be configured")
	}
}

func TestHasUpstreamDetectsMissingUpstream(t *testing.T) {
	repo := initRepo(t)

	ok, err := git.HasUpstream(repo)
	if err != nil {
		t.Fatalf("HasUpstream: %v", err)
	}
	if ok {
		t.Fatal("expected upstream to be missing")
	}
}

func TestInProgressOperationsCleanRepo(t *testing.T) {
	repo := initRepo(t)

	ops, err := git.InProgressOperations(repo)
	if err != nil {
		t.Fatalf("InProgressOperations: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected no in-progress operations, got %v", ops)
	}
}

func TestInProgressOperationsDetectsMerge(t *testing.T) {
	repo := initRepo(t)
	makeCommit(t, repo, "file.txt", "main\n", "main change")
	mustRunGit(t, repo, "checkout", "-b", "feature", "HEAD~1")
	makeCommit(t, repo, "file.txt", "feature\n", "feature change")

	cmd := exec.Command("git", "merge", "main")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected merge conflict, got success: %s", out)
	}

	ops, err := git.InProgressOperations(repo)
	if err != nil {
		t.Fatalf("InProgressOperations: %v", err)
	}
	if len(ops) != 1 || ops[0] != "merge" {
		t.Fatalf("expected merge operation, got %v", ops)
	}
}

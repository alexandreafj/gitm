package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunUpdate_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runUpdate(nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

func TestRunUpdate_SkipsDirtyTracked(t *testing.T) {
	database = setupTestDB(t)
	repo, dir := newRepo(t, database, "repo1")

	writeFile(t, dir, "README.md", "changed\n")

	if err := runUpdate(nil); err != nil {
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

	if err := runUpdate(nil); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
}

func TestRunUpdate_RepoFlag_SingleRepo(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir, origin1, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir, origin2, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	pushRemoteChange(t, origin1, "from-remote1.txt")
	pushRemoteChange(t, origin2, "from-remote2.txt")

	if err := runUpdate([]string{"repo1"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo1Dir, "from-remote1.txt")); err != nil {
		t.Fatal("expected from-remote1.txt in repo1 after update")
	}
	if _, err := os.Stat(filepath.Join(repo2Dir, "from-remote2.txt")); err == nil {
		t.Fatal("repo2 should not have been updated")
	}
}

func TestRunUpdate_RepoFlag_MultipleRepos(t *testing.T) {
	database = setupTestDB(t)

	repo1Dir, origin1, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	repo2Dir, origin2, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	repo3Dir, origin3, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo3", "repo3", repo3Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	pushRemoteChange(t, origin1, "from-remote1.txt")
	pushRemoteChange(t, origin2, "from-remote2.txt")
	pushRemoteChange(t, origin3, "from-remote3.txt")

	if err := runUpdate([]string{"repo1", "repo3"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo1Dir, "from-remote1.txt")); err != nil {
		t.Fatal("expected from-remote1.txt in repo1")
	}
	if _, err := os.Stat(filepath.Join(repo2Dir, "from-remote2.txt")); err == nil {
		t.Fatal("repo2 should not have been updated")
	}
	if _, err := os.Stat(filepath.Join(repo3Dir, "from-remote3.txt")); err != nil {
		t.Fatal("expected from-remote3.txt in repo3")
	}
}

func TestRunUpdate_RepoFlag_UnknownAlias(t *testing.T) {
	database = setupTestDB(t)
	_, _ = newRepo(t, database, "repo1")

	err := runUpdate([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdate_RepoFlag_EmptySlice(t *testing.T) {
	database = setupTestDB(t)

	if err := runUpdate([]string{}); err != nil {
		t.Fatalf("runUpdate with empty slice: %v", err)
	}
}

func TestRunUpdate_ReturnsErrorOnFailure(t *testing.T) {
	database = setupTestDB(t)

	if _, err := database.AddRepository("broken", "broken", "/nonexistent/path/broken", "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runUpdate(nil)
	if err == nil {
		t.Fatal("expected error when repo git operations fail, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update") {
		t.Errorf("error = %q, want to contain \"failed to update\"", err.Error())
	}
}

func TestRunUpdate_DeletedRemoteBranchFallsBackToDefault(t *testing.T) {
	database = setupTestDB(t)
	repoDir, originDir, _ := initRepoWithRemote(t)

	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// Create a feature branch, push it, then delete it from origin.
	mustRunGit(t, repoDir, "checkout", "-b", "feature/old")
	writeFile(t, repoDir, "feature.txt", "feature\n")
	mustRunGit(t, repoDir, "add", "feature.txt")
	mustRunGit(t, repoDir, "commit", "-m", "feature commit")
	mustRunGit(t, repoDir, "push", "--set-upstream", "origin", "feature/old")

	// Delete the remote branch (simulates PR merge + branch cleanup).
	mustRunGit(t, originDir, "branch", "-D", "feature/old")

	// runUpdate should detect the deleted remote branch and switch to default.
	if err := runUpdate([]string{"repo1"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}

	branch := gitCurrentBranch(t, repoDir)
	if branch != "main" {
		t.Errorf("expected to be on default branch 'main', got %q", branch)
	}
}

func pushRemoteChange(t *testing.T, origin, filename string) {
	t.Helper()
	clone := cloneRepo(t, origin)
	mustRunGit(t, clone, "config", "user.email", "test@example.com")
	mustRunGit(t, clone, "config", "user.name", "Test User")
	writeFile(t, clone, filename, "remote content\n")
	mustRunGit(t, clone, "add", filename)
	mustRunGit(t, clone, "commit", "-m", "add "+filename)
	mustRunGit(t, clone, "push")
}

func TestUpdate_NoUpstreamSkippedNotFailed(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	// A branch with no upstream — e.g. created locally and never pushed.
	mustRunGit(t, repoDir, "checkout", "-b", "feature/no-upstream")
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runUpdate(nil); err != nil {
		t.Fatalf("update of a branch with no upstream should skip, not fail: %v", err)
	}

	if branch := gitCurrentBranch(t, repoDir); branch != "feature/no-upstream" {
		t.Errorf("update must not switch branches; expected feature/no-upstream, got %q", branch)
	}
}

func TestRunUpdateRefreshesDefaultOnlyForDeletedUpstreamFallback(t *testing.T) {
	database = setupTestDB(t)
	repo := addRepoWithRemoteDefault(t, "repo1", "main", "master")
	mustRunGit(t, repo.Path, "checkout", "-b", "feature/old")
	writeFile(t, repo.Path, "feature.txt", "feature\n")
	mustRunGit(t, repo.Path, "add", "feature.txt")
	mustRunGit(t, repo.Path, "commit", "-m", "feature commit")
	mustRunGit(t, repo.Path, "push", "--set-upstream", "origin", "feature/old")
	originDir := mustRunGit(t, repo.Path, "remote", "get-url", "origin")
	mustRunGit(t, originDir, "branch", "-D", "feature/old")

	if err := runUpdate([]string{"repo1"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if branch := gitCurrentBranch(t, repo.Path); branch != "master" {
		t.Fatalf("current branch = %q, want refreshed master", branch)
	}
	stored, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "master" {
		t.Fatalf("stored default branch = %q, want master", stored.DefaultBranch)
	}
}

func TestRunUpdateDoesNotRefreshWhenFallbackIsNotNeeded(t *testing.T) {
	database = setupTestDB(t)
	_ = addRepoWithRemoteDefault(t, "repo1", "main", "master")

	if err := runUpdate([]string{"repo1"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	stored, err := database.GetRepository("repo1")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if stored.DefaultBranch != "main" {
		t.Fatalf("ordinary update refreshed default branch to %q", stored.DefaultBranch)
	}
}

func TestRunUpdateStreamsFastResultBeforeSlowPullFinishes(t *testing.T) {
	database = setupTestDB(t)
	fastRepo, _, _ := initRepoWithRemote(t)
	slowRepo, slowOrigin, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("fast", "fast", fastRepo, "main"); err != nil {
		t.Fatalf("AddRepository fast: %v", err)
	}
	if _, err := database.AddRepository("slow", "slow", slowRepo, "main"); err != nil {
		t.Fatalf("AddRepository slow: %v", err)
	}

	releaseFile := filepath.Join(t.TempDir(), "release-upload-pack")
	t.Cleanup(func() {
		if err := os.WriteFile(releaseFile, []byte("release\n"), 0o644); err != nil {
			t.Errorf("release delayed upload-pack during cleanup: %v", err)
		}
	})
	uploadPack := filepath.Join(t.TempDir(), "delayed-upload-pack")
	script := fmt.Sprintf("#!/bin/sh\nwhile test ! -e %s; do sleep 0.01; done\nexec git-upload-pack %s\n", shellQuote(releaseFile), shellQuote(slowOrigin))
	if err := os.WriteFile(uploadPack, []byte(script), 0o755); err != nil {
		t.Fatalf("write delayed upload-pack: %v", err)
	}
	mustRunGit(t, slowRepo, "config", "protocol.ext.allow", "always")
	mustRunGit(t, slowRepo, "remote", "set-url", "origin", "ext::"+uploadPack)

	reader, writer := io.Pipe()
	lines := make(chan string, 32)
	readDone := make(chan struct{})
	var output strings.Builder
	go func() {
		defer close(readDone)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line)
			output.WriteByte('\n')
			lines <- line
		}
		close(lines)
	}()

	runDone := make(chan error, 1)
	go func() {
		runDone <- runUpdateWithGroupTo([]string{"fast", "slow"}, "", writer)
		_ = writer.Close()
	}()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("update output closed before fast repository completed:\n%s", output.String())
			}
			if strings.Contains(line, "[fast") {
				goto fastVisible
			}
		case <-deadline:
			t.Fatal("fast repository result was not streamed while slow pull was blocked")
		}
	}

fastVisible:
	select {
	case err := <-runDone:
		t.Fatalf("update finished before slow pull was released: %v", err)
	default:
	}
	if err := os.WriteFile(releaseFile, []byte("release\n"), 0o644); err != nil {
		t.Fatalf("release slow upload-pack: %v", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("runUpdateWithGroupTo: %v", err)
	}
	<-readDone

	got := output.String()
	if strings.Count(got, "Done:") != 1 {
		t.Fatalf("summary count = %d, want 1; output:\n%s", strings.Count(got, "Done:"), got)
	}
	if strings.Count(got, "[fast") != 1 || strings.Count(got, "[slow") != 1 {
		t.Fatalf("repository results were duplicated or omitted:\n%s", got)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

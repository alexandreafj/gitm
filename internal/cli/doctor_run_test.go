package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDoctor_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runDoctor(nil); err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
}

func TestRunDoctor_CleanRepo(t *testing.T) {
	database = setupTestDB(t)
	repoDir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runDoctor(nil); err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
}

func TestRunDoctor_MissingPathReturnsError(t *testing.T) {
	database = setupTestDB(t)
	missingDir := filepath.Join(t.TempDir(), "missing")
	if _, err := database.AddRepository("missing", "missing", missingDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runDoctor(nil)
	if err == nil {
		t.Fatal("expected missing path to return an error")
	}
	if !strings.Contains(err.Error(), "repository health check failed") {
		t.Fatalf("expected health-check error, got %v", err)
	}
}

func TestRunDoctor_RepoFlagFiltersRepos(t *testing.T) {
	database = setupTestDB(t)

	goodDir, _, _ := initRepoWithRemote(t)
	if _, err := database.AddRepository("good", "good", goodDir, "main"); err != nil {
		t.Fatalf("AddRepository good: %v", err)
	}

	missingDir := filepath.Join(t.TempDir(), "missing")
	if _, err := database.AddRepository("missing", "missing", missingDir, "main"); err != nil {
		t.Fatalf("AddRepository missing: %v", err)
	}

	if err := runDoctor([]string{"good"}); err != nil {
		t.Fatalf("runDoctor with --repo good: %v", err)
	}
}

func TestRunDoctor_RepoFlagUnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	err := runDoctor([]string{"ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
}

func TestInspectRepoHealth_CleanRepoIsOK(t *testing.T) {
	repoDir, _, _ := initRepoWithRemote(t)
	repo, _ := newRepo(t, setupTestDB(t), "repo1")
	repo.Path = repoDir

	report := inspectRepoHealth(repo)
	if report.hasErrors() {
		t.Fatalf("expected clean repo to have no errors, got %#v", report.checks)
	}
	if report.hasWarnings() {
		t.Fatalf("expected clean repo with origin/upstream to have no warnings, got %#v", report.checks)
	}
}

func TestInspectRepoHealth_LocalRepoWithoutOriginWarns(t *testing.T) {
	repoDir := initRepo(t)
	repo, _ := newRepo(t, setupTestDB(t), "repo1")
	repo.Path = repoDir

	report := inspectRepoHealth(repo)
	if report.hasErrors() {
		t.Fatalf("expected local repo to have no errors, got %#v", report.checks)
	}
	if !report.hasWarnings() {
		t.Fatalf("expected local repo without origin/upstream to warn, got %#v", report.checks)
	}
}

func TestInspectRepoHealth_DirtyRepoWarns(t *testing.T) {
	repoDir, _, _ := initRepoWithRemote(t)
	writeFile(t, repoDir, "dirty.txt", "dirty\n")
	repo, _ := newRepo(t, setupTestDB(t), "repo1")
	repo.Path = repoDir

	report := inspectRepoHealth(repo)
	if report.hasErrors() {
		t.Fatalf("expected dirty repo to have no errors, got %#v", report.checks)
	}
	if !report.hasWarnings() {
		t.Fatalf("expected dirty repo to warn, got %#v", report.checks)
	}
}

func TestInspectRepoHealth_DetachedHeadWarns(t *testing.T) {
	repoDir, _, _ := initRepoWithRemote(t)
	commit := mustRunGit(t, repoDir, "rev-parse", "HEAD")
	mustRunGit(t, repoDir, "checkout", "--detach", commit)
	repo, _ := newRepo(t, setupTestDB(t), "repo1")
	repo.Path = repoDir

	report := inspectRepoHealth(repo)
	if report.hasErrors() {
		t.Fatalf("expected detached repo to have no errors, got %#v", report.checks)
	}
	if !report.hasWarnings() {
		t.Fatalf("expected detached repo to warn, got %#v", report.checks)
	}
}

func TestInspectRepoHealth_MissingPathErrors(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "gone")
	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	repo, _ := newRepo(t, setupTestDB(t), "repo1")
	repo.Path = repoDir

	report := inspectRepoHealth(repo)
	if !report.hasErrors() {
		t.Fatalf("expected missing path to error, got %#v", report.checks)
	}
}

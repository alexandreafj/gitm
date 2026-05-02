package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestRunCommit_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runCommit(false, nil); err == nil {
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

	if err := runCommit(false, nil); err != nil {
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
	if err := runCommitWithUI(ui, false, nil); err != nil {
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

	mustRunGit(t, repoDir, "checkout", "-b", "AA-111")

	var seenBranch string
	ui := fakeUI{branchSeen: &seenBranch}
	if err := runCommitWithUI(ui, true, nil); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}

	if seenBranch != "AA-111" {
		t.Fatalf("branchSeen = %q, want %q", seenBranch, "AA-111")
	}

	msg := mustRunGit(t, repoDir, "log", "-1", "--format=%s")
	if msg != "AA-111 test commit" {
		t.Fatalf("commit message = %q, want %q", msg, "AA-111 test commit")
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
	if err := runCommitWithUI(ui, true, nil); err != nil {
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
	if err := runCommitWithUI(ui, true, nil); err != nil {
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
	if err := runCommitWithUI(ui, true, nil); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}
}

func TestRunCommit_BranchLookupFailureWarnsAndContinues(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	var seenBranch string
	ui := fakeUI{commitMsg: "add new feature", branchSeen: &seenBranch}
	branchErr := errors.New("branch lookup failed")

	originalStdout := os.Stdout
	originalColorOutput := color.Output
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w
	color.Output = w

	runErr := runCommitWithBranchLookup(ui, true, nil, func(string) (string, error) {
		return "", branchErr
	})

	_ = w.Close()
	os.Stdout = originalStdout
	color.Output = originalColorOutput

	var output bytes.Buffer
	if _, copyErr := io.Copy(&output, r); copyErr != nil {
		t.Fatalf("io.Copy: %v", copyErr)
	}
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runCommitWithBranchLookup: %v", runErr)
	}

	if seenBranch != "" {
		t.Fatalf("branchSeen = %q, want empty string", seenBranch)
	}

	msg := mustRunGit(t, repoDir, "log", "-1", "--format=%s")
	if msg != "add new feature" {
		t.Fatalf("commit message = %q, want %q", msg, "add new feature")
	}

	if !strings.Contains(output.String(), "Cannot detect current branch") {
		t.Fatalf("expected warning in output, got %q", output.String())
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

func TestRunCommit_RepoFlag_TargetsOnlySpecifiedRepos(t *testing.T) {
	database = setupTestDB(t)

	// repo1 — dirty, on feature branch
	repo1Dir := initRepo(t)
	mustRunGit(t, repo1Dir, "checkout", "-b", "feature/AA-1")
	writeFile(t, repo1Dir, "change1.txt", "dirty\n")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	// repo2 — dirty, on feature branch
	repo2Dir := initRepo(t)
	mustRunGit(t, repo2Dir, "checkout", "-b", "feature/AA-1")
	writeFile(t, repo2Dir, "change2.txt", "dirty\n")
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}

	// repo3 — dirty, on feature branch — NOT in --repo list
	repo3Dir := initRepo(t)
	mustRunGit(t, repo3Dir, "checkout", "-b", "feature/AA-1")
	writeFile(t, repo3Dir, "change3.txt", "dirty\n")
	if _, err := database.AddRepository("repo3", "repo3", repo3Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo3: %v", err)
	}

	ui := fakeUI{commitMsg: "add feature"}
	if err := runCommitWithUI(ui, true, []string{"repo1", "repo2"}); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}

	// repo1 and repo2 should have a new commit beyond the initial one.
	log1 := mustRunGit(t, repo1Dir, "log", "--oneline")
	if !strings.Contains(log1, "add feature") {
		t.Errorf("repo1: expected commit message containing 'add feature', got: %s", log1)
	}
	log2 := mustRunGit(t, repo2Dir, "log", "--oneline")
	if !strings.Contains(log2, "add feature") {
		t.Errorf("repo2: expected commit message containing 'add feature', got: %s", log2)
	}

	// repo3 must NOT have been committed — still dirty.
	log3 := mustRunGit(t, repo3Dir, "log", "--oneline")
	if strings.Contains(log3, "add feature") {
		t.Errorf("repo3: should NOT have been committed (not in --repo list), got: %s", log3)
	}
}

func TestRunCommit_RepoFlag_UnknownAliasErrors(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	ui := fakeUI{}
	err := runCommitWithUI(ui, true, []string{"repo1", "ghost-repo"})
	if err == nil {
		t.Fatal("expected error for unknown alias, got nil")
	}
}

func TestRunCommit_RepoFlag_SkipsNonDirtyRepoSilently(t *testing.T) {
	database = setupTestDB(t)

	// repo1 — clean (no uncommitted changes)
	repo1Dir := initRepo(t)
	mustRunGit(t, repo1Dir, "checkout", "-b", "feature/AA-1")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	ui := fakeUI{commitMsg: "should not be called"}
	if err := runCommitWithUI(ui, true, []string{"repo1"}); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}

	// Confirm no extra commit was made — only the initial commit exists.
	log1 := mustRunGit(t, repo1Dir, "log", "--oneline")
	if strings.Contains(log1, "should not be called") {
		t.Errorf("repo1: unexpected commit in clean repo: %s", log1)
	}
}

func TestRunCommit_RepoFlag_AllDirtyProtected(t *testing.T) {
	database = setupTestDB(t)

	// repo1 — dirty but on default branch (protected)
	repo1Dir := initRepo(t)
	writeFile(t, repo1Dir, "change.txt", "dirty\n")
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}

	ui := fakeUI{commitMsg: "should not be called"}
	if err := runCommitWithUI(ui, true, []string{"repo1"}); err != nil {
		t.Fatalf("runCommitWithUI: %v", err)
	}

	// Confirm no commit was made.
	log1 := mustRunGit(t, repo1Dir, "log", "--oneline")
	if strings.Contains(log1, "should not be called") {
		t.Errorf("repo1: unexpected commit on protected branch: %s", log1)
	}
}

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

	if err := runCommit(false); err == nil {
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

	if err := runCommit(false); err != nil {
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
	if err := runCommitWithUI(ui, false); err != nil {
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
	if err := runCommitWithUI(ui, true); err != nil {
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
	if err := runCommitWithUI(ui, true); err != nil {
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
	if err := runCommitWithUI(ui, true); err != nil {
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
	if err := runCommitWithUI(ui, true); err != nil {
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

	runErr := runCommitWithBranchLookup(ui, true, func(string) (string, error) {
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

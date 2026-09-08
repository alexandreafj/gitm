package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunDiscardUsesRawStatusForExactFilename(t *testing.T) {
	database := setupTestDB(t)
	repoDir := initRepo(t)
	if _, err := database.AddRepository("exact", "exact", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	const filename = "space quoted ü.txt"
	writeFile(t, repoDir, filename, "original\n")
	mustRunGit(t, repoDir, "add", "--", filename)
	mustRunGit(t, repoDir, "commit", "-m", "add exact filename")
	writeFile(t, repoDir, filename, "discard me\n")

	if err := runDiscardWithUI(fakeUI{}, []string{"exact"}); err != nil {
		t.Fatalf("runDiscardWithUI: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(repoDir, filename))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(content); got != "original\n" {
		t.Errorf("selected file content = %q, want original content", got)
	}
}

func TestRunDiscardRestoresRawRenameEntry(t *testing.T) {
	database := setupTestDB(t)
	repoDir := initRepo(t)
	if _, err := database.AddRepository("rename", "rename", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	const renamed = "renamed draft.md"
	mustRunGit(t, repoDir, "mv", "README.md", renamed)
	if err := runDiscardWithUI(fakeUI{}, []string{"rename"}); err != nil {
		t.Fatalf("runDiscardWithUI: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, renamed)); !os.IsNotExist(err) {
		t.Errorf("rename destination still exists: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile restored source: %v", err)
	}
	if got := string(content); got != "# test repo\n" {
		t.Errorf("restored source content = %q, want original README", got)
	}
}

func TestRunDiscardDryRunPreservesRawRenameAndPreviewsExactCommands(t *testing.T) {
	database := setupTestDB(t)
	repoDir := initRepo(t)
	if _, err := database.AddRepository("preview", "preview", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	const renamed = "renamed draft.md"
	mustRunGit(t, repoDir, "mv", "README.md", renamed)
	selectedDir := filepath.Join(repoDir, "scratch dir")
	if err := os.MkdirAll(selectedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, selectedDir, "file.txt", "untracked\n")
	before := mustRunGit(t, repoDir, "status", "--porcelain=v1", "-z")

	var runErr error
	output := captureOutput(t, func() {
		runErr = runDiscardWithUIDryRun(fakeUI{}, []string{"preview"}, true)
	})
	if runErr != nil {
		t.Fatalf("runDiscardWithUIDryRun: %v", runErr)
	}
	if after := mustRunGit(t, repoDir, "status", "--porcelain=v1", "-z"); after != before {
		t.Errorf("dry run changed repository status:\nbefore %q\nafter  %q", before, after)
	}

	wantActions := []string{
		`git reset HEAD -- ':(literal)renamed draft.md' README.md`,
		`git checkout -- README.md`,
		`git clean -fd -- ':(literal)renamed draft.md' ':(literal)scratch dir/'`,
	}
	for _, want := range wantActions {
		if !strings.Contains(output, want) {
			t.Errorf("preview missing %q:\n%s", want, output)
		}
	}
	if strings.ContainsRune(output, '\x00') {
		t.Errorf("preview contains raw NUL separator: %q", output)
	}
}

func TestDiscardDryRunActionsShellQuotesLiteralFilename(t *testing.T) {
	actions := discardDryRunActions([]string{"?? price$'draft*.txt"})
	want := `git clean -fd -- ':(literal)price$'"'"'draft*.txt'`
	if len(actions) != 1 || actions[0] != want {
		t.Fatalf("discardDryRunActions() = %q, want [%q]", actions, want)
	}
}

func TestDiscardDryRunActionsCopyTouchesOnlyDestination(t *testing.T) {
	actions := discardDryRunActions([]string{"C  copy.txt\x00source.txt"})
	want := []string{
		"git reset HEAD -- copy.txt",
		"git clean -fd -- copy.txt",
	}
	if !slices.Equal(actions, want) {
		t.Fatalf("discardDryRunActions() = %q, want %q", actions, want)
	}
}

func TestRunDiscardDryRunRejectsUnselectedRenameSourceCollision(t *testing.T) {
	database := setupTestDB(t)
	repoDir := initRepo(t)
	if _, err := database.AddRepository("collision", "collision", repoDir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	mustRunGit(t, repoDir, "mv", "README.md", "destination.md")
	writeFile(t, repoDir, "README.md", "untracked replacement\n")
	selected := "R  destination.md\x00README.md"
	before := mustRunGit(t, repoDir, "status", "--porcelain=v1", "-z")

	var runErr error
	output := captureOutput(t, func() {
		runErr = runDiscardWithUIDryRun(fakeUI{fileSelect: []string{selected}}, []string{"collision"}, true)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "failed to discard") {
		t.Fatalf("runDiscardWithUIDryRun() error = %v, want failed discard", runErr)
	}
	if !strings.Contains(output, "README.md") || !strings.Contains(output, "already exists") {
		t.Errorf("dry-run output missing collision context:\n%s", output)
	}
	if strings.Contains(output, "Would run:") {
		t.Errorf("dry-run preview advertised destructive commands despite collision:\n%s", output)
	}
	if after := mustRunGit(t, repoDir, "status", "--porcelain=v1", "-z"); after != before {
		t.Errorf("dry run mutated repository status:\nbefore %q\nafter  %q", before, after)
	}
}

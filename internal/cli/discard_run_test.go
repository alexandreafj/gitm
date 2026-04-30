package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Sentinel errors matching the strings checked in discard.go.
var (
	errCanceled        = fmt.Errorf("canceled")
	errNoFilesSelected = fmt.Errorf("no files selected")
)

func TestRunDiscard_NoRepos(t *testing.T) {
	database = setupTestDB(t)

	if err := runDiscardWithUI(fakeUI{}, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}
}

func TestRunDiscard_NoDirtyRepos(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	if err := runDiscardWithUI(fakeUI{}, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}
}

func TestRunDiscard_DirtyRepos(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	// fakeUI.FileSelect returns all porcelain lines by default.
	if err := runDiscardWithUI(fakeUI{}, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	// dirty.txt should have been cleaned (untracked → git clean -f).
	if _, statErr := os.Stat(filepath.Join(repoDir, "dirty.txt")); !os.IsNotExist(statErr) {
		t.Error("dirty.txt should have been removed after discard")
	}
}

// TestRunDiscard_FileSelection verifies that only selected files are discarded.
func TestRunDiscard_FileSelection(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// Create 3 dirty files: 1 tracked modification + 2 untracked.
	writeFile(t, repoDir, "README.md", "modified readme\n") // tracked
	writeFile(t, repoDir, "new1.txt", "new file 1\n")       // untracked
	writeFile(t, repoDir, "new2.txt", "new file 2\n")       // untracked (survivor)

	// Only select README.md and new1.txt — new2.txt should survive.
	ui := fakeUI{
		fileSelect: []string{
			" M README.md",
			"?? new1.txt",
		},
	}

	if err := runDiscardWithUI(ui, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	// README.md should be reverted to original.
	content, readErr := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if readErr != nil {
		t.Fatalf("read README.md: %v", readErr)
	}
	if string(content) != "# test repo\n" {
		t.Errorf("README.md = %q, want %q", string(content), "# test repo\n")
	}

	// new1.txt should be gone.
	if _, statErr := os.Stat(filepath.Join(repoDir, "new1.txt")); !os.IsNotExist(statErr) {
		t.Error("new1.txt should have been removed")
	}

	// new2.txt should survive (not selected).
	if _, statErr := os.Stat(filepath.Join(repoDir, "new2.txt")); statErr != nil {
		t.Error("new2.txt should still exist — it was not selected")
	}
}

// TestRunDiscard_RepoFlag verifies --repo bypasses MultiSelect.
func TestRunDiscard_RepoFlag(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("myrepo", "myrepo", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "dirty\n")

	// Pass --repo alias. fakeUI.FileSelect returns all lines by default.
	if err := runDiscardWithUI(fakeUI{}, []string{"myrepo"}); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	// dirty.txt should be gone.
	if _, statErr := os.Stat(filepath.Join(repoDir, "dirty.txt")); !os.IsNotExist(statErr) {
		t.Error("dirty.txt should have been removed")
	}
}

// TestRunDiscard_RepoFlagUnknownAlias verifies --repo with unknown alias returns error.
func TestRunDiscard_RepoFlagUnknownAlias(t *testing.T) {
	database = setupTestDB(t)

	err := runDiscardWithUI(fakeUI{}, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown repo alias")
	}
}

// TestRunDiscard_CancelFileSelect verifies that canceling file selection skips the repo.
func TestRunDiscard_CancelFileSelect(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "should survive\n")

	ui := fakeUI{
		fileErr: errCanceled,
	}

	// Should NOT return error — canceling is graceful.
	if err := runDiscardWithUI(ui, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	// dirty.txt should still exist (discard was canceled).
	if _, statErr := os.Stat(filepath.Join(repoDir, "dirty.txt")); statErr != nil {
		t.Error("dirty.txt should still exist after cancel")
	}
}

// TestRunDiscard_NoFilesSelected verifies that selecting no files skips the repo.
func TestRunDiscard_NoFilesSelected(t *testing.T) {
	database = setupTestDB(t)
	repoDir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repoDir, "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	writeFile(t, repoDir, "dirty.txt", "should survive\n")

	ui := fakeUI{
		fileErr: errNoFilesSelected,
	}

	if err := runDiscardWithUI(ui, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	// dirty.txt should still exist.
	if _, statErr := os.Stat(filepath.Join(repoDir, "dirty.txt")); statErr != nil {
		t.Error("dirty.txt should still exist when no files selected")
	}
}

// TestRunDiscard_MultipleRepos verifies discard works across multiple repos.
func TestRunDiscard_MultipleRepos(t *testing.T) {
	database = setupTestDB(t)

	// Set up two repos, both dirty.
	repo1Dir := initRepo(t)
	_, err := database.AddRepository("repo1", "repo1", repo1Dir, "main")
	if err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	writeFile(t, repo1Dir, "dirty1.txt", "dirty\n")

	repo2Dir := initRepo(t)
	_, err = database.AddRepository("repo2", "repo2", repo2Dir, "main")
	if err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}
	writeFile(t, repo2Dir, "dirty2.txt", "dirty\n")

	// fakeUI returns all repos and all files by default.
	if err := runDiscardWithUI(fakeUI{}, nil); err != nil {
		t.Fatalf("runDiscard: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(repo1Dir, "dirty1.txt")); !os.IsNotExist(statErr) {
		t.Error("repo1/dirty1.txt should have been removed")
	}
	if _, statErr := os.Stat(filepath.Join(repo2Dir, "dirty2.txt")); !os.IsNotExist(statErr) {
		t.Error("repo2/dirty2.txt should have been removed")
	}
}

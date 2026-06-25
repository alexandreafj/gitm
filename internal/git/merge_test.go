package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestMerge(t *testing.T) {
	dir := initRepo(t)

	// Branch off main, then advance main with a new file.
	mustRunGit(t, dir, "checkout", "-b", "feature")
	mustRunGit(t, dir, "checkout", "main")
	makeCommit(t, dir, "added.go", "package main\n", "add on main")

	// Merge main into feature; feature should gain added.go.
	mustRunGit(t, dir, "checkout", "feature")
	if _, err := git.Merge(dir, "main"); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "added.go")); err != nil {
		t.Errorf("expected added.go to be merged into feature: %v", err)
	}

	conflicts, err := git.UnmergedFiles(dir)
	if err != nil {
		t.Fatalf("UnmergedFiles: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts after clean merge, got %v", conflicts)
	}
}

func TestMergeConflict(t *testing.T) {
	dir := initRepo(t)

	// Both branches modify the same file differently → guaranteed conflict.
	makeCommit(t, dir, "shared.txt", "base\n", "base content")
	mustRunGit(t, dir, "checkout", "-b", "feature")
	makeCommit(t, dir, "shared.txt", "feature change\n", "feature edit")

	mustRunGit(t, dir, "checkout", "main")
	makeCommit(t, dir, "shared.txt", "main change\n", "main edit")

	mustRunGit(t, dir, "checkout", "feature")
	if _, err := git.Merge(dir, "main"); err == nil {
		t.Fatal("expected Merge to fail on conflicting changes")
	}

	conflicts, err := git.UnmergedFiles(dir)
	if err != nil {
		t.Fatalf("UnmergedFiles: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0] != "shared.txt" {
		t.Errorf("expected [shared.txt] unmerged, got %v", conflicts)
	}

	// The repo must be left in a merging state for manual resolution.
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err != nil {
		t.Errorf("expected MERGE_HEAD to exist (merge left in place): %v", err)
	}
}

func TestUnmergedFilesClean(t *testing.T) {
	dir := initRepo(t)

	conflicts, err := git.UnmergedFiles(dir)
	if err != nil {
		t.Fatalf("UnmergedFiles: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no unmerged files in clean repo, got %v", conflicts)
	}
}

func TestIsMerging_Clean(t *testing.T) {
	dir := initRepo(t)

	merging, err := git.IsMerging(dir)
	if err != nil {
		t.Fatalf("IsMerging: %v", err)
	}
	if merging {
		t.Error("expected IsMerging=false on clean repo")
	}
}

func TestIsMerging_DuringConflict(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "shared.txt", "base\n", "base content")
	mustRunGit(t, dir, "checkout", "-b", "feature")
	makeCommit(t, dir, "shared.txt", "feature change\n", "feature edit")

	mustRunGit(t, dir, "checkout", "main")
	makeCommit(t, dir, "shared.txt", "main change\n", "main edit")

	mustRunGit(t, dir, "checkout", "feature")
	if _, err := git.Merge(dir, "main"); err == nil {
		t.Fatal("expected Merge to fail on conflicting changes")
	}

	merging, err := git.IsMerging(dir)
	if err != nil {
		t.Fatalf("IsMerging: %v", err)
	}
	if !merging {
		t.Error("expected IsMerging=true during merge conflict")
	}
}

func TestCommitMerge(t *testing.T) {
	dir := initRepo(t)

	makeCommit(t, dir, "shared.txt", "base\n", "base content")
	mustRunGit(t, dir, "checkout", "-b", "feature")
	makeCommit(t, dir, "shared.txt", "feature change\n", "feature edit")

	mustRunGit(t, dir, "checkout", "main")
	makeCommit(t, dir, "shared.txt", "main change\n", "main edit")

	mustRunGit(t, dir, "checkout", "feature")
	if _, err := git.Merge(dir, "main"); err == nil {
		t.Fatal("expected Merge to fail on conflicting changes")
	}

	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("resolved\n"), 0644); err != nil {
		t.Fatalf("write resolution: %v", err)
	}
	if err := git.StageFiles(dir, []string{"UU shared.txt"}); err != nil {
		t.Fatalf("StageFiles: %v", err)
	}

	out, err := git.CommitMerge(dir, "resolve merge conflict")
	if err != nil {
		t.Fatalf("CommitMerge: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty commit output")
	}

	merging, err := git.IsMerging(dir)
	if err != nil {
		t.Fatalf("IsMerging after commit: %v", err)
	}
	if merging {
		t.Error("expected IsMerging=false after CommitMerge")
	}

	conflicts, err := git.UnmergedFiles(dir)
	if err != nil {
		t.Fatalf("UnmergedFiles: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no unmerged files after merge commit, got %v", conflicts)
	}
}

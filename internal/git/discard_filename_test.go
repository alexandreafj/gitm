package git_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestDiscardFilesUsesExactTrackedFilenames(t *testing.T) {
	filenames := []struct {
		name               string
		unsupportedWindows bool
	}{
		{name: "space name.txt"},
		{name: "unicodé 文件.txt"},
		{name: `quote"name.txt`, unsupportedWindows: true},
		{name: " leading and trailing .txt ", unsupportedWindows: true},
	}

	for _, filename := range filenames {
		t.Run(filename.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && filename.unsupportedWindows {
				t.Skip("filename contains characters unsupported by Windows")
			}
			repo := initRepo(t)
			makeCommit(t, repo, filename.name, "original\n", "add exact filename")
			writeFile(t, repo, filename.name, "discard me\n")
			writeFile(t, repo, "README.md", "preserve me\n")

			entries, err := git.DirtyFilesWithRawStatus(repo)
			if err != nil {
				t.Fatalf("DirtyFilesWithRawStatus: %v", err)
			}
			selected := " M " + filename.name
			if !slices.Contains(entries, selected) {
				t.Fatalf("DirtyFilesWithRawStatus() = %q, want entry %q", entries, selected)
			}
			if err := git.DiscardFiles(repo, []string{selected}); err != nil {
				t.Fatalf("DiscardFiles(%q) from entries %q: %v", selected, entries, err)
			}

			if got := readFileContent(t, repo, filename.name); got != "original\n" {
				t.Errorf("selected file content = %q, want original content", got)
			}
			if got := readFileContent(t, repo, "README.md"); got != "preserve me\n" {
				t.Errorf("unselected file content = %q, want preserved modification", got)
			}
		})
	}
}

func TestDiscardFilesTreatsWildcardFilenameLiterally(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames cannot contain an asterisk")
	}
	repo := initRepo(t)
	makeCommit(t, repo, "wild*.txt", "selected original\n", "add wildcard filename")
	makeCommit(t, repo, "wild-other.txt", "other original\n", "add wildcard neighbor")
	writeFile(t, repo, "wild*.txt", "discard me\n")
	writeFile(t, repo, "wild-other.txt", "preserve me\n")

	entries, err := git.DirtyFilesWithRawStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithRawStatus: %v", err)
	}
	selected := " M wild*.txt"
	if !slices.Contains(entries, selected) {
		t.Fatalf("DirtyFilesWithRawStatus() = %q, want entry %q", entries, selected)
	}
	if err := git.DiscardFiles(repo, []string{selected}); err != nil {
		t.Fatalf("DiscardFiles(%q) from entries %q: %v", selected, entries, err)
	}

	if got := readFileContent(t, repo, "wild*.txt"); got != "selected original\n" {
		t.Errorf("selected wildcard file content = %q, want original content", got)
	}
	if got := readFileContent(t, repo, "wild-other.txt"); got != "preserve me\n" {
		t.Errorf("wildcard neighbor content = %q, want preserved modification", got)
	}
}

func TestDiscardFilesRestoresRenameAndPreservesOtherChanges(t *testing.T) {
	repo := initRepo(t)
	makeCommit(t, repo, "staged-other.txt", "staged original\n", "add staged neighbor")
	makeCommit(t, repo, "worktree-other.txt", "worktree original\n", "add worktree neighbor")
	writeFile(t, repo, "staged-other.txt", "preserve staged\n")
	mustRunGit(t, repo, "add", "staged-other.txt")
	writeFile(t, repo, "worktree-other.txt", "preserve worktree\n")

	const renamed = "renamed draft.md"
	mustRunGit(t, repo, "mv", "README.md", renamed)
	entries, err := git.DirtyFilesWithRawStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithRawStatus: %v", err)
	}
	selected := "R  " + renamed + "\x00README.md"
	if !slices.Contains(entries, selected) {
		t.Fatalf("DirtyFilesWithRawStatus() = %q, want entry %q", entries, selected)
	}
	if err := git.DiscardFiles(repo, []string{selected}); err != nil {
		t.Fatalf("DiscardFiles(%q) from entries %q: %v", selected, entries, err)
	}

	if got := readFileContent(t, repo, "README.md"); got != "# test repo\n" {
		t.Errorf("restored source content = %q, want original README", got)
	}
	if _, err := os.Stat(filepath.Join(repo, renamed)); !os.IsNotExist(err) {
		t.Errorf("rename destination still exists: %v", err)
	}
	if got := mustRunGit(t, repo, "diff", "--cached", "--name-only"); got != "staged-other.txt" {
		t.Errorf("staged files = %q, want staged-other.txt preserved", got)
	}
	if got := readFileContent(t, repo, "worktree-other.txt"); got != "preserve worktree\n" {
		t.Errorf("unselected worktree content = %q, want preserved modification", got)
	}
}

func TestDiscardFilesCopyPreservesUnselectedSource(t *testing.T) {
	repo := initRepo(t)
	const original = "line one\nline two\nline three\nline four\n"
	makeCommit(t, repo, "source.txt", original, "add copy source")
	mustRunGit(t, repo, "config", "status.renames", "copies")
	writeFile(t, repo, "copy.txt", original)
	const modified = "source changed\nline one\nline two\nline three\nline four\n"
	writeFile(t, repo, "source.txt", modified)
	mustRunGit(t, repo, "add", ".")

	entries, err := git.DirtyFilesWithRawStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithRawStatus: %v", err)
	}
	selected := "C  copy.txt\x00source.txt"
	if !slices.Contains(entries, selected) || !slices.Contains(entries, "M  source.txt") {
		t.Fatalf("DirtyFilesWithRawStatus() = %q, want copy plus separately modified source", entries)
	}
	if err := git.DiscardFiles(repo, []string{selected}); err != nil {
		t.Fatalf("DiscardFiles(%q): %v", selected, err)
	}

	if _, err := os.Stat(filepath.Join(repo, "copy.txt")); !os.IsNotExist(err) {
		t.Errorf("selected copy still exists: %v", err)
	}
	if got := readFileContent(t, repo, "source.txt"); got != modified {
		t.Errorf("unselected source content = %q, want preserved modification %q", got, modified)
	}
	if got := mustRunGit(t, repo, "diff", "--cached", "--name-only"); got != "source.txt" {
		t.Errorf("staged files = %q, want source.txt preserved", got)
	}
}

func TestDiscardFilesRejectsUnselectedRenameSourceCollisionBeforeMutation(t *testing.T) {
	repo := initRepo(t)
	makeCommit(t, repo, "source.txt", "original\n", "add rename source")
	mustRunGit(t, repo, "mv", "source.txt", "destination.txt")
	writeFile(t, repo, "source.txt", "untracked replacement\n")
	selected := "R  destination.txt\x00source.txt"
	before := mustRunGit(t, repo, "status", "--porcelain=v1", "-z")

	err := git.ValidateDiscardFiles(repo, []string{selected})
	if err == nil || !strings.Contains(err.Error(), "source.txt") {
		t.Fatalf("ValidateDiscardFiles() error = %v, want source collision", err)
	}
	if err := git.DiscardFiles(repo, []string{selected}); err == nil || !strings.Contains(err.Error(), "source.txt") {
		t.Fatalf("DiscardFiles() error = %v, want source collision", err)
	}

	if after := mustRunGit(t, repo, "status", "--porcelain=v1", "-z"); after != before {
		t.Errorf("failed discard mutated repository status:\nbefore %q\nafter  %q", before, after)
	}
	if got := readFileContent(t, repo, "source.txt"); got != "untracked replacement\n" {
		t.Errorf("unselected replacement content = %q, want unchanged", got)
	}
}

func TestDiscardFilesAllowsSelectedRenameSourceCollision(t *testing.T) {
	repo := initRepo(t)
	makeCommit(t, repo, "source.txt", "original\n", "add rename source")
	mustRunGit(t, repo, "mv", "source.txt", "destination.txt")
	writeFile(t, repo, "source.txt", "selected replacement\n")

	selected := []string{
		"R  destination.txt\x00source.txt",
		"?? source.txt",
	}
	if err := git.DiscardFiles(repo, selected); err != nil {
		t.Fatalf("DiscardFiles(): %v", err)
	}

	if got := readFileContent(t, repo, "source.txt"); got != "original\n" {
		t.Errorf("restored source content = %q, want HEAD content", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "destination.txt")); !os.IsNotExist(err) {
		t.Errorf("rename destination still exists: %v", err)
	}
}

func TestDiscardFilesRemovesOnlySelectedUntrackedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows directory names cannot contain an asterisk")
	}
	repo := initRepo(t)
	selectedDir := filepath.Join(repo, "scratch*")
	neighborDir := filepath.Join(repo, "scratch-keep")
	if err := os.MkdirAll(selectedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll selected directory: %v", err)
	}
	if err := os.MkdirAll(neighborDir, 0o755); err != nil {
		t.Fatalf("MkdirAll neighbor directory: %v", err)
	}
	writeFile(t, selectedDir, "selected.txt", "discard me\n")
	writeFile(t, neighborDir, "neighbor.txt", "preserve me\n")

	entries, err := git.DirtyFilesWithRawStatus(repo)
	if err != nil {
		t.Fatalf("DirtyFilesWithRawStatus: %v", err)
	}
	selected := "?? scratch*/"
	if !slices.Contains(entries, selected) {
		t.Fatalf("DirtyFilesWithRawStatus() = %q, want entry %q", entries, selected)
	}
	if err := git.DiscardFiles(repo, []string{selected}); err != nil {
		t.Fatalf("DiscardFiles(%q) from entries %q: %v", selected, entries, err)
	}

	if _, err := os.Stat(selectedDir); !os.IsNotExist(err) {
		t.Errorf("selected directory still exists: %v", err)
	}
	if _, err := os.Stat(neighborDir); err != nil {
		t.Errorf("unselected wildcard neighbor was removed: %v", err)
	}
}

// Package git provides helpers to execute git operations on local repositories.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// run executes a git command in the given directory and returns stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// IsGitRepo reports whether the directory is the root of a git repository.
func IsGitRepo(path string) bool {
	out, err := run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	// Confirm the root matches the supplied path (handles nested dirs).
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	repoRoot, err := filepath.Abs(out)
	if err != nil {
		return false
	}
	return abs == repoRoot
}

// DefaultBranch detects the default branch (main/master) for a repo.
// It tries origin/HEAD first, then falls back to probing local branches.
func DefaultBranch(path string) (string, error) {
	// Try origin/HEAD symbolic ref (most reliable).
	out, err := run(path, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// refs/remotes/origin/main → main
		parts := strings.Split(strings.TrimSpace(out), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fall back: check which of main/master exists locally.
	for _, candidate := range []string{"main", "master"} {
		_, err := run(path, "rev-parse", "--verify", candidate)
		if err == nil {
			return candidate, nil
		}
	}

	// Last resort: use HEAD's current branch.
	out, err = run(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "main", nil // sane default
	}
	return out, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(path string) (string, error) {
	return run(path, "rev-parse", "--abbrev-ref", "HEAD")
}

// IsDirty reports whether the working tree has uncommitted changes.
func IsDirty(path string) (bool, error) {
	out, err := run(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// DirtyFiles returns the list of modified/untracked files.
func DirtyFiles(path string) ([]string, error) {
	out, err := run(path, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var files []string
	for _, l := range lines {
		if l != "" {
			files = append(files, strings.TrimSpace(l))
		}
	}
	return files, nil
}

// Checkout switches to the specified branch.
func Checkout(path, branch string) error {
	_, err := run(path, "checkout", branch)
	return err
}

// Pull runs git pull on the current branch.
func Pull(path string) (string, error) {
	return run(path, "pull", "--ff-only")
}

// CreateBranch creates and checks out a new branch from the current HEAD.
func CreateBranch(path, branch string) error {
	_, err := run(path, "checkout", "-b", branch)
	return err
}

// BranchExists reports whether a local branch with the given name exists.
func BranchExists(path, branch string) bool {
	_, err := run(path, "rev-parse", "--verify", branch)
	return err == nil
}

// RemoteBranchExists reports whether a remote tracking branch exists.
func RemoteBranchExists(path, branch string) bool {
	_, err := run(path, "ls-remote", "--exit-code", "--heads", "origin", branch)
	return err == nil
}

// RenameBranch renames a local branch from oldName to newName.
func RenameBranch(path, oldName, newName string) error {
	_, err := run(path, "branch", "-m", oldName, newName)
	return err
}

// DeleteRemoteBranch deletes a branch on origin.
func DeleteRemoteBranch(path, branch string) error {
	_, err := run(path, "push", "origin", "--delete", branch)
	return err
}

// PushBranch pushes a local branch to origin and sets upstream tracking.
func PushBranch(path, branch string) error {
	_, err := run(path, "push", "--set-upstream", "origin", branch)
	return err
}

// AheadBehind returns how many commits the current branch is ahead/behind origin.
// AheadBehind returns how many commits the current branch is ahead/behind origin.
// Pass fetch=true to run git fetch first for accurate up-to-date numbers (slower).
func AheadBehind(path string, fetch bool) (ahead, behind int, err error) {
	if fetch {
		_, _ = run(path, "fetch", "--quiet")
	}

	out, err := run(path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		// No upstream tracking branch — treat as 0/0.
		return 0, 0, nil
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}
	fmt.Sscanf(parts[0], "%d", &ahead)
	fmt.Sscanf(parts[1], "%d", &behind)
	return ahead, behind, nil
}

// RepoName returns the base directory name of a repository path.
func RepoName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(abs)
}

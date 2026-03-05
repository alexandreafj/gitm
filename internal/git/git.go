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
	return strings.TrimRight(stdout.String(), "\r\n"), nil
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
		_, checkErr := run(path, "rev-parse", "--verify", candidate)
		if checkErr == nil {
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

// IsDirty reports whether the working tree has uncommitted changes,
// including untracked files.
func IsDirty(path string) (bool, error) {
	out, err := run(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// IsDirtyTrackedOnly reports whether tracked files have modifications or
// staged changes. Untracked files are ignored (-uno flag).
// Use this for pull/checkout where untracked files pose no risk of conflict.
func IsDirtyTrackedOnly(path string) (bool, error) {
	out, err := run(path, "status", "--porcelain", "-uno")
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

// DiscardChanges discards all uncommitted changes in the working tree and
// removes untracked files. This is equivalent to:
//
//	git checkout -- .
//	git clean -fd
//
// This is irreversible — call IsDirty first to confirm there are changes.
func DiscardChanges(path string) error {
	// Discard modifications to tracked files.
	if _, err := run(path, "checkout", "--", "."); err != nil {
		return fmt.Errorf("discard tracked changes: %w", err)
	}
	// Remove untracked files and directories.
	if _, err := run(path, "clean", "-fd"); err != nil {
		return fmt.Errorf("clean untracked files: %w", err)
	}
	return nil
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
// Pass fetch=true to run git fetch first for accurate up-to-date numbers (slower).
func AheadBehind(path string, fetch bool) (ahead, behind int, err error) {
	if fetch {
		//nolint:errcheck // fetch failure should not block ahead/behind check
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
	//nolint:errcheck // sscanf errors are safe to ignore; vars stay 0 on failure
	_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
	//nolint:errcheck // sscanf errors are safe to ignore; vars stay 0 on failure
	_, _ = fmt.Sscanf(parts[1], "%d", &behind)
	return ahead, behind, nil
}

// StageFiles stages specific files (by their path relative to the repo root).
func StageFiles(path string, files []string) error {
	// Strip the porcelain status prefix (e.g. " M ", "?? ") to get the raw path.
	cleaned := make([]string, 0, len(files))
	for _, f := range files {
		// porcelain format: "XY filename" where XY is two chars + space.
		if len(f) > 3 {
			cleaned = append(cleaned, strings.TrimSpace(f[3:]))
		} else {
			cleaned = append(cleaned, strings.TrimSpace(f))
		}
	}
	args := append([]string{"add", "--"}, cleaned...)
	_, err := run(path, args...)
	return err
}

// Commit creates a commit with the given message.
func Commit(path, message string) (string, error) {
	return run(path, "commit", "-m", message)
}

// Push pushes the current branch to origin.
// If no upstream is set yet, it sets one automatically.
func Push(path string) error {
	branch, err := CurrentBranch(path)
	if err != nil {
		return fmt.Errorf("get branch: %w", err)
	}
	_, err = run(path, "push", "--set-upstream", "origin", branch)
	return err
}

// IsDefaultBranch reports whether the current branch equals the repo's default branch.
func IsDefaultBranch(path, defaultBranch string) (bool, error) {
	current, err := CurrentBranch(path)
	if err != nil {
		return false, err
	}
	return current == defaultBranch, nil
}

// DirtyFilesWithStatus returns modified/untracked files keeping the full
// porcelain line (e.g. " M src/foo.php", "?? scratch.txt").
func DirtyFilesWithStatus(path string) ([]string, error) {
	out, err := run(path, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// StashPush stashes all uncommitted changes (tracked and untracked) with an
// auto-generated message. Pass an empty message to use git's default.
func StashPush(path, message string) error {
	args := []string{"stash", "push", "--include-untracked"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := run(path, args...)
	return err
}

// StashApply applies the most recent stash without removing it.
func StashApply(path string) error {
	_, err := run(path, "stash", "apply")
	return err
}

// StashPop applies the most recent stash and removes it from the stash list.
func StashPop(path string) error {
	_, err := run(path, "stash", "pop")
	return err
}

// StashList returns the stash entries for the repository (one line per entry).
// Returns nil if there are no stash entries.
func StashList(path string) ([]string, error) {
	out, err := run(path, "stash", "list")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var entries []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			entries = append(entries, l)
		}
	}
	return entries, nil
}

// HasStash reports whether the repository has any stash entries.
func HasStash(path string) (bool, error) {
	entries, err := StashList(path)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// RepoName returns the base directory name of a repository path.
func RepoName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(abs)
}

// ResetSoft moves HEAD back by <ref> (e.g. "HEAD~1") while keeping all
// changes staged in the index. Equivalent to: git reset --soft <ref>
func ResetSoft(path, ref string) error {
	_, err := run(path, "reset", "--soft", ref)
	return err
}

// ResetMixed moves HEAD back by <ref> and unstages all changes, leaving them
// as working-tree modifications. Equivalent to: git reset <ref>
func ResetMixed(path, ref string) error {
	_, err := run(path, "reset", ref)
	return err
}

// ResetHard moves HEAD back by <ref> and discards all staged and working-tree
// changes. This is irreversible. Equivalent to: git reset --hard <ref>
func ResetHard(path, ref string) error {
	_, err := run(path, "reset", "--hard", ref)
	return err
}

// CommitLog returns the last n commits as one-line entries (hash + subject).
// Each entry has the format "<short-hash> <subject>".
func CommitLog(path string, n int) ([]string, error) {
	if n <= 0 {
		n = 1
	}
	out, err := run(path, "log", fmt.Sprintf("-n%d", n), "--oneline", "--no-decorate")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var entries []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			entries = append(entries, l)
		}
	}
	return entries, nil
}

// ForcePush pushes the current branch to origin using --force-with-lease,
// which refuses to overwrite if the remote has commits we haven't seen.
// This is the safest form of force-push for history rewriting.
func ForcePush(path string) error {
	branch, err := CurrentBranch(path)
	if err != nil {
		return fmt.Errorf("get branch: %w", err)
	}
	_, err = run(path, "push", "--force-with-lease", "origin", branch)
	return err
}

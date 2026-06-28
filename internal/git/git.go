// Package git provides helpers to execute git operations on local repositories.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
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
	// EvalSymlinks is used on both sides so that macOS /var → /private/var
	// symlinks (and similar) do not cause false negatives.
	abs, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Fall back to Abs if the path cannot be resolved (e.g. doesn't exist).
		abs, err = filepath.Abs(path)
		if err != nil {
			return false
		}
	}
	repoRoot, err := filepath.EvalSymlinks(out)
	if err != nil {
		repoRoot, err = filepath.Abs(out)
		if err != nil {
			return false
		}
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

// DiscardFiles selectively discards uncommitted changes for the given files.
// Each entry in porcelainFiles is a porcelain-format line (e.g. " M foo.go",
// "?? bar.txt", "A  new.go"). The function groups files by status and runs
// the appropriate git command for each group:
//
//   - Staged new files (A): git reset HEAD -- <files>, then git clean -fd -- <files>
//   - Tracked modifications/deletions (M, D, etc.): git reset HEAD -- <files>,
//     then git checkout -- <files> (reset first to unstage any staged changes)
//   - Untracked files/directories (??): git clean -fd -- <files>
//
// The -d flag is required because git status --porcelain collapses untracked
// directories into a single "?? dir/" entry, and git clean without -d refuses
// to remove directories.
//
// This is irreversible.
func DiscardFiles(path string, porcelainFiles []string) error {
	if len(porcelainFiles) == 0 {
		return nil
	}

	var staged []string    // "A " — newly staged files
	var tracked []string   // " M", "M ", "MM", " D", "D ", etc. — tracked modifications
	var untracked []string // "??" — untracked files

	for _, line := range porcelainFiles {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		filePath := strings.TrimSpace(line[3:])
		if filePath == "" {
			continue
		}

		switch {
		case status == "??":
			untracked = append(untracked, filePath)
		case status[0] == 'A':
			// Staged new file: index says Added, work-tree may or may not differ.
			staged = append(staged, filePath)
		default:
			// Everything else (M, D, R, etc.) — tracked file with changes.
			tracked = append(tracked, filePath)
		}
	}

	// Unstage and remove staged new files.
	if len(staged) > 0 {
		resetArgs := append([]string{"reset", "HEAD", "--"}, staged...)
		if _, err := run(path, resetArgs...); err != nil {
			return fmt.Errorf("reset staged files: %w", err)
		}
		cleanArgs := append([]string{"clean", "-fd", "--"}, staged...)
		if _, err := run(path, cleanArgs...); err != nil {
			return fmt.Errorf("clean staged files: %w", err)
		}
	}

	// Revert tracked modifications/deletions: reset index first (handles
	// staged modifications like "M " or "MM"), then checkout to restore
	// working-tree state to match HEAD.
	if len(tracked) > 0 {
		resetArgs := append([]string{"reset", "HEAD", "--"}, tracked...)
		if _, err := run(path, resetArgs...); err != nil {
			return fmt.Errorf("reset tracked files: %w", err)
		}
		checkoutArgs := append([]string{"checkout", "--"}, tracked...)
		if _, err := run(path, checkoutArgs...); err != nil {
			return fmt.Errorf("discard tracked changes: %w", err)
		}
	}

	// Remove untracked files and directories.
	if len(untracked) > 0 {
		cleanArgs := append([]string{"clean", "-fd", "--"}, untracked...)
		if _, err := run(path, cleanArgs...); err != nil {
			return fmt.Errorf("clean untracked files: %w", err)
		}
	}

	return nil
}

// Pull runs git pull on the current branch.
func Pull(path string) (string, error) {
	return run(path, "pull", "--ff-only")
}

// Merge merges ref (e.g. "origin/main" or "main") into the current branch and
// returns git's output. --no-edit suppresses the merge-commit message editor,
// which would otherwise hang the non-interactive multi-repo runner. On a merge
// conflict git exits non-zero and leaves the working tree in a merging state;
// callers detect that case with UnmergedFiles rather than treating it as a hard
// failure.
func Merge(path, ref string) (string, error) {
	return run(path, "merge", "--no-edit", ref)
}

// UnmergedFiles returns the paths with merge conflicts (unmerged index entries).
// A non-empty result after a failed Merge means the merge stopped on conflicts
// and left the tree in a conflicted state for manual resolution. git diff exits
// zero here, so the signal survives even though run() drops stdout on a failed
// merge.
func UnmergedFiles(path string) ([]string, error) {
	out, err := run(path, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			files = append(files, strings.TrimSpace(l))
		}
	}
	return files, nil
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

// FetchBranch fetches a single branch from origin so that git checkout can
// create a local tracking branch from the remote ref.
// The -- separator ensures the branch name is always treated as a refspec
// and never misinterpreted as a flag (e.g. if it starts with -).
func FetchBranch(path, branch string) error {
	_, err := run(path, "fetch", "origin", "--", branch)
	return err
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

// DeleteLocalBranch deletes a local branch. When force is false it uses
// `git branch -d`, which refuses to delete branches with unmerged commits.
func DeleteLocalBranch(path, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(path, "branch", flag, branch)
	return err
}

// BranchMerged reports whether branch is already reachable from HEAD.
func BranchMerged(path, branch string) (bool, error) {
	_, err := run(path, "merge-base", "--is-ancestor", branch, "HEAD")
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
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

// TrackedFiles returns all tracked files in the repository as porcelain-style
// lines with a " T " prefix (e.g. " T src/main.go") for display in the file picker.
func TrackedFiles(path string) ([]string, error) {
	out, err := run(path, "ls-files")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, " T "+l)
		}
	}
	return files, nil
}

// UntrackedFiles returns all untracked, non-ignored files as porcelain-style
// lines (e.g. "?? scratch.txt") for display in the file picker.
func UntrackedFiles(path string) ([]string, error) {
	out, err := run(path, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, "?? "+l)
		}
	}
	return files, nil
}

// UntrackFiles removes files from the git index but keeps them on disk.
// Equivalent to: git rm --cached -- <files>
func UntrackFiles(path string, files []string) error {
	args := append([]string{"rm", "--cached", "--"}, cleanPorcelainPaths(files)...)
	_, err := run(path, args...)
	return err
}

// cleanPorcelainPaths strips the two-char porcelain status prefix (e.g. " M ",
// "?? ") from each line, yielding the bare repo-relative path.
func cleanPorcelainPaths(files []string) []string {
	cleaned := make([]string, 0, len(files))
	for _, f := range files {
		// porcelain format: "XY filename" where XY is two chars + space.
		if len(f) > 3 {
			cleaned = append(cleaned, strings.TrimSpace(f[3:]))
		} else {
			cleaned = append(cleaned, strings.TrimSpace(f))
		}
	}
	return cleaned
}

// StageFiles stages specific files (by their path relative to the repo root).
func StageFiles(path string, files []string) error {
	args := append([]string{"add", "--"}, cleanPorcelainPaths(files)...)
	_, err := run(path, args...)
	return err
}

// Commit creates a commit containing only the given files. Scoping the commit to
// an explicit pathspec stops files that were already staged but NOT selected by the
// user from leaking into the commit. Callers must pass a non-empty file list; an
// empty list would degrade to a whole-index commit.
func Commit(path, message string, files []string) (string, error) {
	args := append([]string{"commit", "-m", message, "--"}, cleanPorcelainPaths(files)...)
	return run(path, args...)
}

// CommitMerge completes a merge commit without a pathspec. Git forbids partial
// commits during a merge, so this commits the entire staged index.
func CommitMerge(path, message string) (string, error) {
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

// RemoteConfigured reports whether a named remote exists in the repository.
func RemoteConfigured(path, remote string) (bool, error) {
	_, err := run(path, "remote", "get-url", remote)
	if err != nil {
		if strings.Contains(err.Error(), "No such remote") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HasUpstream reports whether the current branch has an upstream configured.
func HasUpstream(path string) (bool, error) {
	_, err := run(path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no upstream configured") ||
			strings.Contains(msg, "HEAD does not point to a branch") ||
			strings.Contains(msg, "ambiguous argument '@{upstream}'") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// InProgressOperations returns git operations that are currently unfinished.
func InProgressOperations(path string) ([]string, error) {
	checks := []struct {
		gitPath string
		label   string
		isDir   bool
	}{
		{gitPath: "MERGE_HEAD", label: "merge"},
		{gitPath: "rebase-merge", label: "rebase", isDir: true},
		{gitPath: "rebase-apply", label: "rebase", isDir: true},
		{gitPath: "CHERRY_PICK_HEAD", label: "cherry-pick"},
		{gitPath: "REVERT_HEAD", label: "revert"},
		{gitPath: "BISECT_LOG", label: "bisect"},
	}

	var ops []string
	seen := make(map[string]bool)
	for _, check := range checks {
		gitPath, err := run(path, "rev-parse", "--git-path", check.gitPath)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(gitPath) {
			gitPath = filepath.Join(path, gitPath)
		}

		info, err := os.Stat(gitPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if check.isDir && !info.IsDir() {
			continue
		}
		if !check.isDir && info.IsDir() {
			continue
		}
		if !seen[check.label] {
			ops = append(ops, check.label)
			seen[check.label] = true
		}
	}
	return ops, nil
}

// IsMerging reports whether the repository is in the middle of a merge.
func IsMerging(path string) (bool, error) {
	gitPath, err := run(path, "rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return false, fmt.Errorf("check merge state: %w", err)
	}
	if !filepath.IsAbs(gitPath) {
		gitPath = filepath.Join(path, gitPath)
	}
	_, err = os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat MERGE_HEAD: %w", err)
	}
	return true, nil
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

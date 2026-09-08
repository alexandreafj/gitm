// Package git provides helpers to execute git operations on local repositories.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const remoteDefaultBranchTimeout = 10 * time.Second

// run executes a git command in the given directory and returns stdout.
func run(dir string, args ...string) (string, error) {
	return runContext(context.Background(), dir, nil, args...)
}

func runContext(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Force the C locale so git's messages stay in English regardless of the
	// user's LANG/LC_ALL: several callers match on message text (e.g. "no
	// upstream configured", "Your local changes", "Already up to date").
	// os/exec keeps the last duplicate key, so this append wins.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
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

// RemoteDefaultBranch returns origin's advertised default branch.
func RemoteDefaultBranch(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteDefaultBranchTimeout)
	defer cancel()
	return RemoteDefaultBranchContext(ctx, path)
}

// RemoteDefaultBranchContext returns origin's advertised default branch and
// stops the lookup when ctx is canceled.
func RemoteDefaultBranchContext(ctx context.Context, path string) (string, error) {
	out, err := runContext(ctx, path, []string{"GIT_TERMINAL_PROMPT=0"}, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", fmt.Errorf("query remote default branch: %w", err)
	}

	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "ref: ") {
			continue
		}
		if !strings.HasSuffix(line, "\tHEAD") {
			return "", fmt.Errorf("remote default branch: malformed symbolic HEAD output: %q", line)
		}

		ref := strings.TrimSuffix(strings.TrimPrefix(line, "ref: "), "\tHEAD")
		const branchRefPrefix = "refs/heads/"
		if !strings.HasPrefix(ref, branchRefPrefix) || len(ref) == len(branchRefPrefix) {
			return "", fmt.Errorf("remote default branch: malformed symbolic HEAD output: %q", line)
		}
		return strings.TrimPrefix(ref, branchRefPrefix), nil
	}

	return "", errors.New("remote default branch: missing symbolic HEAD output")
}

// DefaultBranch detects the default branch (main/master) for a repo.
// It tries the remote symbolic HEAD first, then falls back to local refs.
func DefaultBranch(path string) (string, error) {
	if branch, err := RemoteDefaultBranch(path); err == nil {
		return branch, nil
	}

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
	ref, err := run(path, "rev-parse", "--symbolic-full-name", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	return strings.TrimPrefix(ref, "refs/heads/"), nil
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

// ValidateDiscardFiles rejects selected renames that would overwrite an
// existing source path which the user did not also select for discard.
func ValidateDiscardFiles(path string, porcelainFiles []string) error {
	selectedPaths := make(map[string]struct{}, len(porcelainFiles))
	var renameSources []string
	for _, line := range porcelainFiles {
		if len(line) < 4 {
			continue
		}
		paths := cleanPorcelainPaths([]string{line})
		if len(paths) == 0 {
			continue
		}
		selectedPaths[paths[0]] = struct{}{}
		if strings.Contains(line[:2], "R") && len(paths) > 1 {
			renameSources = append(renameSources, paths[1:]...)
		}
	}

	for _, source := range renameSources {
		if _, selected := selectedPaths[source]; selected {
			continue
		}
		_, err := os.Lstat(filepath.Join(path, filepath.FromSlash(source)))
		switch {
		case err == nil:
			return fmt.Errorf("rename source %q already exists and is not selected for discard", source)
		case os.IsNotExist(err):
			continue
		default:
			return fmt.Errorf("check rename source %q: %w", source, err)
		}
	}
	return nil
}

// DiscardFiles selectively discards entries from DirtyFilesWithRawStatus.
// Rename and copy entries include the destination followed by the source,
// separated by NUL. Literal pathspecs keep filename metacharacters exact.
func DiscardFiles(path string, porcelainFiles []string) error {
	if len(porcelainFiles) == 0 {
		return nil
	}
	if err := ValidateDiscardFiles(path, porcelainFiles); err != nil {
		return err
	}

	var resetPaths []string
	var checkoutPaths []string
	var cleanPaths []string
	renameSources := make(map[string]struct{})
	for _, line := range porcelainFiles {
		if len(line) < 4 || !strings.Contains(line[:2], "R") {
			continue
		}
		paths := cleanPorcelainPaths([]string{line})
		for _, source := range paths[1:] {
			renameSources[source] = struct{}{}
		}
	}

	for _, line := range porcelainFiles {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		paths := cleanPorcelainPaths([]string{line})
		if len(paths) == 0 {
			continue
		}

		switch {
		case status == "??":
			if _, restoredByRename := renameSources[paths[0]]; !restoredByRename {
				cleanPaths = append(cleanPaths, paths[0])
			}
		case strings.Contains(status, "R") && len(paths) > 1:
			resetPaths = append(resetPaths, paths...)
			checkoutPaths = append(checkoutPaths, paths[1:]...)
			cleanPaths = append(cleanPaths, paths[0])
		case strings.Contains(status, "C") && len(paths) > 1:
			resetPaths = append(resetPaths, paths[0])
			cleanPaths = append(cleanPaths, paths[0])
		case status[0] == 'A':
			resetPaths = append(resetPaths, paths[0])
			cleanPaths = append(cleanPaths, paths[0])
		default:
			resetPaths = append(resetPaths, paths[0])
			checkoutPaths = append(checkoutPaths, paths[0])
		}
	}

	literalPathspecs := func(paths []string) []string {
		pathspecs := make([]string, 0, len(paths))
		for _, filePath := range paths {
			pathspecs = append(pathspecs, ":(literal)"+filePath)
		}
		return pathspecs
	}

	if len(resetPaths) > 0 {
		resetArgs := append([]string{"reset", "HEAD", "--"}, literalPathspecs(resetPaths)...)
		if _, err := run(path, resetArgs...); err != nil {
			return fmt.Errorf("reset selected files: %w", err)
		}
	}

	if len(checkoutPaths) > 0 {
		checkoutArgs := append([]string{"checkout", "--"}, literalPathspecs(checkoutPaths)...)
		if _, err := run(path, checkoutArgs...); err != nil {
			return fmt.Errorf("discard tracked changes: %w", err)
		}
	}

	if len(cleanPaths) > 0 {
		cleanArgs := append([]string{"clean", "-fd", "--"}, literalPathspecs(cleanPaths)...)
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

// IsNoUpstreamError reports whether err came from a git command that failed
// because the current branch has no upstream tracking information (e.g.
// `git pull` on a branch that was never pushed). Callers treat this as a
// normal state — there is simply nothing to pull — rather than a failure.
func IsNoUpstreamError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no tracking information")
}

// PullRebase fetches origin/<branch> and rebases the current branch onto it,
// autostashing any uncommitted changes around the rebase. It is used to recover
// a branch whose push was rejected because the remote advanced (a
// non-fast-forward): rebasing the local commits onto the fetched remote tip
// leaves the branch strictly ahead so a subsequent Push fast-forwards origin.
//
// On rebase conflicts git exits non-zero and leaves the working tree in a
// rebasing state; callers detect that with UnmergedFiles rather than treating
// it as a hard failure (mirrors Merge).
func PullRebase(path, branch string) (string, error) {
	return run(path, "pull", "--rebase", "--autostash", "origin", branch)
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

// Fetch refreshes all remote-tracking refs from origin (git fetch). The branch
// dashboard calls it once per repo under --fetch so that remote branch existence
// and ahead/behind numbers reflect the current remote state.
func Fetch(path string) error {
	_, err := run(path, "fetch")
	return err
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
	return BranchMergedInto(path, branch, "HEAD")
}

// BranchMergedInto reports whether branch is reachable from target — i.e. every
// commit on branch is already contained in target. Both arguments may be any ref
// (branch name, refs/heads/…, refs/remotes/origin/…). git merge-base exits 1 when
// branch is not an ancestor, which is a clean "not merged" answer rather than an
// error.
func BranchMergedInto(path, branch, target string) (bool, error) {
	_, err := run(path, "merge-base", "--is-ancestor", branch, target)
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

// AheadBehind compares the current branch with its configured upstream.
// A branch without an upstream returns zero counts; failed queries return errors.
func AheadBehind(path string, fetch bool) (ahead, behind int, err error) {
	if fetch {
		if err := Fetch(path); err != nil {
			return 0, 0, fmt.Errorf("fetch before ahead/behind: %w", err)
		}
	}
	branch, err := CurrentBranch(path)
	if err != nil {
		return 0, 0, fmt.Errorf("current branch for ahead/behind: %w", err)
	}
	return AheadBehindOf(path, branch)
}

// AheadBehindOf compares a local branch with its configured upstream.
// Missing tracking configuration returns zero counts; a missing configured ref
// is an error, because its counts are unknown.
func AheadBehindOf(path, ref string) (ahead, behind int, err error) {
	upstream, err := Upstream(path, ref)
	if err != nil {
		return 0, 0, fmt.Errorf("upstream for %s: %w", ref, err)
	}
	if upstream == "" {
		return 0, 0, nil
	}
	branchRef := "refs/heads/" + ref
	out, err := run(path, "rev-list", "--left-right", "--count", branchRef+"..."+ref+"@{upstream}", "--")
	if err != nil {
		return 0, 0, fmt.Errorf("ahead/behind for %s: %w", ref, err)
	}
	if len(strings.Fields(out)) != 2 {
		return 0, 0, fmt.Errorf("invalid ahead/behind counts %q", out)
	}
	if _, err := fmt.Sscanf(out, "%d %d", &ahead, &behind); err != nil {
		return 0, 0, fmt.Errorf("parse ahead/behind counts: %w", err)
	}
	if ahead < 0 || behind < 0 {
		return 0, 0, fmt.Errorf("negative ahead/behind counts %q", out)
	}
	return ahead, behind, nil
}

// Upstream returns the configured upstream even when its tracking ref is gone.
// A local branch without tracking configuration returns an empty string.
func Upstream(path, branch string) (string, error) {
	if branch == "HEAD" {
		if _, err := run(path, "rev-parse", "--verify", "HEAD"); err != nil {
			return "", fmt.Errorf("verify detached HEAD: %w", err)
		}
		return "", nil
	}
	ref := "refs/heads/" + branch
	if _, err := run(path, "check-ref-format", ref); err != nil {
		return "", fmt.Errorf("validate branch %q: %w", branch, err)
	}
	out, err := run(path, "for-each-ref", "--format=%(refname)%00%(upstream:short)", "--", ref)
	if err != nil {
		return "", fmt.Errorf("read upstream for %s: %w", branch, err)
	}
	for _, record := range strings.Split(out, "\n") {
		name, upstream, ok := strings.Cut(record, "\x00")
		if ok && name == ref {
			if upstream == "" {
				configured, err := hasTrackingConfiguration(path, branch)
				if err != nil {
					return "", err
				}
				if configured {
					return "", fmt.Errorf("configured upstream for %s cannot be resolved", branch)
				}
			}
			return upstream, nil
		}
	}
	return "", fmt.Errorf("local branch %q not found", branch)
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
// "?? ") from each line, yielding the bare repo-relative paths. Rename and
// copy entries from DirtyFilesWithRawStatus contain both paths separated by NUL.
func cleanPorcelainPaths(files []string) []string {
	cleaned := make([]string, 0, len(files))
	for _, f := range files {
		var paths string
		if len(f) > 3 {
			paths = f[3:]
		} else {
			paths = strings.TrimSpace(f)
		}
		for _, path := range strings.Split(paths, "\x00") {
			if path != "" {
				cleaned = append(cleaned, path)
			}
		}
	}
	return cleaned
}

func cleanPorcelainCurrentPaths(files []string) []string {
	cleaned := make([]string, 0, len(files))
	for _, f := range files {
		if len(f) > 3 {
			f = f[3:]
		} else {
			f = strings.TrimSpace(f)
		}
		if path, _, found := strings.Cut(f, "\x00"); found {
			f = path
		}
		if f != "" {
			cleaned = append(cleaned, f)
		}
	}
	return cleaned
}

// StageFiles stages specific files (by their path relative to the repo root).
func StageFiles(path string, files []string) error {
	args := append([]string{"add", "--"}, cleanPorcelainCurrentPaths(files)...)
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

// IsNonFastForwardError reports whether a push failed because the remote branch
// advanced and rejected a non-fast-forward update. Git output is forced to the
// C locale by run, so these markers are stable across user locales.
func IsNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "non-fast-forward") ||
		(strings.Contains(msg, "[rejected]") && strings.Contains(msg, "fetch first"))
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

// DirtyFilesWithRawStatus returns porcelain entries with exact, unquoted paths.
// Rename and copy entries store the destination and source paths separated by NUL.
func DirtyFilesWithRawStatus(path string) ([]string, error) {
	out, err := run(path, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	records := strings.Split(out, "\x00")
	files := make([]string, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		if len(record) < 3 {
			return nil, fmt.Errorf("parse porcelain status entry %q", record)
		}

		status := record[:2]
		if strings.ContainsAny(status, "RC") {
			i++
			if i >= len(records) || records[i] == "" {
				return nil, fmt.Errorf("parse porcelain rename or copy entry %q", record)
			}
			record += "\x00" + records[i]
		}
		files = append(files, record)
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

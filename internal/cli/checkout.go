package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

// defaultBranchKeywords are the argument values that trigger "checkout default branch" mode.
var defaultBranchKeywords = map[string]bool{
	"master": true,
	"main":   true,
}

func checkoutCmd() *cobra.Command {
	var (
		repoAliases []string
		groupName   string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "checkout [branch]",
		Short: "Checkout a branch across repositories",
		Long: `Switch repositories to a branch and pull the latest changes.

Three modes of operation:

  gitm checkout
      Interactive: select repos via TUI, then type a branch name.
      Skips repos where the branch doesn't exist.

  gitm checkout master  (or: gitm checkout main)
      Switches ALL repos to their configured default branch and pulls.
      Uses the default branch recorded by Git (origin/HEAD) without contacting
      the remote; only a repository whose ref is missing, dangling, or
      different from the cached default asks origin, and a failed lookup warns
      and uses the cached branch. If a
      checkout or pull fails, gitm asks origin once for its default branch.
      When the remote renamed it, gitm fetches the new branch, records it in
      origin/HEAD and the cache, and retries that repository; a failed
      checkout of an unchanged default is also fetched and retried once.
      Dry-run previews never persist a refreshed default.

  gitm checkout <branch-name>
      Checks out <branch-name> in ALL repos where it exists.
      Branches not present locally are fetched from origin in a single step;
      a branch that exists nowhere is skipped with a warning, while an
      unreachable origin is reported as an error (non-zero exit). A branch
      fetched from origin during checkout is not pulled again.

Repositories are skipped when uncommitted changes conflict with the target branch.
When the requested branch is already current, tracked uncommitted changes skip
the checkout and pull. A clean current branch still pulls remote updates.
If the branch has no upstream, the pull after checkout is skipped with a note
instead of failing (gitm push sets the upstream).

Use --repo to limit the operation to specific repositories by alias.
Use --group to limit the operation to repositories in a group.
Use --dry-run to preview the checkout, fetch, and pull commands without
changing branches or fetching remote-only branches.
When both are provided, gitm checks out only aliases that also belong to the group.`,
		Example: `  gitm checkout
  gitm checkout master
  gitm checkout feature/JIRA-12345
  gitm checkout master --group backend
  gitm checkout master --repo=api-gateway,auth-service
  gitm checkout feature/JIRA-12345 -r api-gateway -g backend
  gitm checkout feature/JIRA-12345 --dry-run`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runCheckoutWithUIDryRun(liveUI{}, args, repoAliases, dryRun)
			}
			return runCheckoutWithUIAndGroupDryRun(liveUI{}, args, repoAliases, groupName, dryRun)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit checkout to specific repository aliases (comma-separated)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Preview checkout and pull operations without changing repositories")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func runCheckoutWithUI(ui ui, args []string, repoAliases []string) error {
	return runCheckoutWithUIAndGroup(ui, args, repoAliases, "")
}

func runCheckoutWithUIAndGroup(ui ui, args []string, repoAliases []string, groupName string) error {
	return runCheckoutWithUIAndGroupDryRun(ui, args, repoAliases, groupName, false)
}

func runCheckoutWithUIDryRun(ui ui, args []string, repoAliases []string, dryRun bool) error {
	return runCheckoutWithUIAndGroupDryRun(ui, args, repoAliases, "", dryRun)
}

func runCheckoutWithUIAndGroupDryRun(ui ui, args []string, repoAliases []string, groupName string, dryRun bool) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}

	// Determine mode from args.
	arg := ""
	if len(args) > 0 {
		arg = strings.TrimSpace(args[0])
	}

	switch {
	case arg == "":
		// Interactive mode.
		return runCheckoutInteractiveDryRun(repos, ui, dryRun)

	case defaultBranchKeywords[strings.ToLower(arg)]:
		// Default branch mode.
		return runCheckoutDefaultDryRun(repos, dryRun)

	default:
		// Specific branch mode.
		return runCheckoutBranchDryRun(repos, arg, dryRun)
	}
}

// runCheckoutDefault switches all repos to their configured default branch and pulls.
func runCheckoutDefault(repos []*db.Repository) error {
	return runCheckoutDefaultDryRun(repos, false)
}

func runCheckoutDefaultDryRun(repos []*db.Repository, dryRun bool) error {
	if err := reconcileDefaultBranches(database, repos, !dryRun); err != nil {
		return fmt.Errorf("refresh default branches: %w", err)
	}

	if dryRun {
		printDryRunPreview(
			fmt.Sprintf("Default branch checkout preview for %d repository(ies)", len(repos)),
			checkoutDefaultDryRunItems(repos),
		)
		return nil
	}

	fmt.Printf("Checking out default branch and pulling for %d repositories…\n\n", len(repos))

	results := runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		return checkoutDefaultInRepo(database, repo)
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to checkout", runner.ErrorCount(results))
	}
	return nil
}

// checkoutDefaultInRepo switches repo to its default branch and pulls. The
// default comes from local metadata, so a failure may mean origin renamed it or
// it was never fetched: origin is then asked once, the live default is fetched
// (and, if renamed, recorded in origin/HEAD and the cache), and the repo is
// retried once. A failed lookup, or a pull failure with an unchanged default,
// returns the original error.
func checkoutDefaultInRepo(database *db.DB, repo *db.Repository) (string, string, error) {
	return checkoutDefaultBranch(database, repo, false, "")
}

// checkoutDefaultBranch runs once more with retry set after a repair; the retry
// prefixes its result with note and never repairs again.
func checkoutDefaultBranch(database *db.DB, repo *db.Repository, retry bool, note string) (string, string, error) {
	skipReason, err := dirtyCurrentBranchSkip(repo.Path, repo.DefaultBranch)
	if err != nil {
		return "", "", err
	}
	if skipReason != "" {
		return "", skipReason, nil
	}

	var failure error
	checkoutFailed := false
	if checkoutErr := git.Checkout(repo.Path, repo.DefaultBranch); checkoutErr != nil {
		if skip, reason := checkoutConflictSkip(repo.Path, checkoutErr); skip {
			return "", reason, nil
		}
		failure = fmt.Errorf("checkout %s: %w", repo.DefaultBranch, checkoutErr)
		checkoutFailed = true
	} else {
		out, pullErr := git.Pull(repo.Path)
		switch {
		case pullErr == nil:
			return fmt.Sprintf("on %s — %s%s", repo.DefaultBranch, note, summarisePull(out)), "", nil
		case git.IsNoUpstreamError(pullErr):
			// A default branch with no upstream (local-only repo) has nothing
			// to pull; the checkout itself succeeded.
			return fmt.Sprintf("on %s — %sno upstream, pull skipped", repo.DefaultBranch, note), "", nil
		}
		failure = fmt.Errorf("pull: %w", pullErr)
		// Local changes blocking the pull say nothing about the default branch.
		if isCheckoutConflict(pullErr) {
			return "", "", failure
		}
	}

	if retry {
		return "", "", failure
	}
	live, lookupErr := git.RemoteDefaultBranch(repo.Path)
	// A pull of an unchanged default already fetched it; retrying would only
	// turn a real pull error into a dirty-branch skip.
	if lookupErr != nil || (live == repo.DefaultBranch && !checkoutFailed) {
		return "", "", failure
	}
	// The retry needs origin/<live> present. A rename that doctor recorded but
	// never fetched reaches this point with live equal to the cached name.
	if err := git.FetchBranch(repo.Path, live); err != nil {
		return "", "", fmt.Errorf("%w; fetch default branch %s: %w", failure, live, err)
	}
	if live == repo.DefaultBranch {
		return checkoutDefaultBranch(database, repo, true, "")
	}
	if err := git.SetRemoteHead(repo.Path, live); err != nil {
		return "", "", fmt.Errorf("%w; origin renamed default branch to %s: %w", failure, live, err)
	}
	if err := database.UpdateDefaultBranch(repo.Alias, live); err != nil {
		return "", "", fmt.Errorf("%w; persist default branch %s for %s: %w", failure, live, repo.Alias, err)
	}
	note = fmt.Sprintf("default branch renamed from %s; ", repo.DefaultBranch)
	repo.DefaultBranch = live
	return checkoutDefaultBranch(database, repo, true, note)
}

// runCheckoutBranch checks out a specific branch in all repos, skipping those
// where the branch does not exist locally or remotely.
func runCheckoutBranch(repos []*db.Repository, branch string) error {
	return runCheckoutBranchDryRun(repos, branch, false)
}

func runCheckoutBranchDryRun(repos []*db.Repository, branch string, dryRun bool) error {
	if dryRun {
		printDryRunPreview(
			fmt.Sprintf("Branch %q checkout preview for %d repository(ies)", branch, len(repos)),
			checkoutBranchDryRunItems(repos, branch),
		)
		return nil
	}

	fmt.Printf("Checking out branch %q in %d repositories…\n\n", branch, len(repos))

	results := runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		return checkoutBranchInRepo(repo, branch)
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to checkout", runner.ErrorCount(results))
	}
	return nil
}

// runCheckoutInteractive lets the user pick repos via TUI, type a branch name,
// then checks out that branch in the selected repos.
func runCheckoutInteractive(repos []*db.Repository, ui ui) error {
	return runCheckoutInteractiveDryRun(repos, ui, false)
}

func runCheckoutInteractiveDryRun(repos []*db.Repository, ui ui, dryRun bool) error {
	chosen, err := ui.MultiSelect(repos, "Select repositories to checkout", false, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	branch, err := ui.BranchNameInput()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if dryRun {
		printDryRunPreview(
			fmt.Sprintf("Branch %q checkout preview for %d repository(ies)", branch, len(chosen)),
			checkoutBranchDryRunItems(chosen, branch),
		)
		return nil
	}

	fmt.Printf("\nChecking out branch %q in %d repositories…\n\n", branch, len(chosen))

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		return checkoutBranchInRepo(repo, branch)
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to checkout", runner.ErrorCount(results))
	}
	return nil
}

// checkoutBranchInRepo performs the dirty check, branch-existence check,
// checkout, and pull for a single repo. Shared by both specific and interactive modes.
// A branch missing locally is fetched directly instead of probed with ls-remote
// first: one round trip, and the fetched tip needs no pull afterwards.
func checkoutBranchInRepo(repo *db.Repository, branch string) (string, string, error) {
	fetched := false
	if !git.BranchExists(repo.Path, branch) {
		if fetchErr := git.FetchBranch(repo.Path, branch); fetchErr != nil {
			if git.IsMissingRemoteRefError(fetchErr) {
				return "", branchNotFoundSkip(branch), nil
			}
			// A repo without origin has no remote to look in, so the branch is
			// simply absent rather than unreachable.
			hasOrigin, remoteErr := git.RemoteConfigured(repo.Path, "origin")
			if remoteErr != nil {
				return "", "", fmt.Errorf("fetch: %w; inspect origin remote: %w", fetchErr, remoteErr)
			}
			if !hasOrigin {
				return "", branchNotFoundSkip(branch), nil
			}
			return "", "", fmt.Errorf("fetch: %w", fetchErr)
		}
		fetched = true
	}

	skipReason, err := dirtyCurrentBranchSkip(repo.Path, branch)
	if err != nil {
		return "", "", err
	}
	if skipReason != "" {
		return "", skipReason, nil
	}

	if checkoutErr := git.Checkout(repo.Path, branch); checkoutErr != nil {
		if skip, reason := checkoutConflictSkip(repo.Path, checkoutErr); skip {
			return "", reason, nil
		}
		return "", "", fmt.Errorf("checkout: %w", checkoutErr)
	}
	if fetched {
		return fmt.Sprintf("on %s — fetched from origin", branch), "", nil
	}

	out, err := git.Pull(repo.Path)
	if err != nil {
		// A local-only branch has nothing to pull; the checkout itself succeeded.
		if git.IsNoUpstreamError(err) {
			return fmt.Sprintf("on %s — no upstream, pull skipped (`gitm push` sets one)", branch), "", nil
		}
		return "", "", fmt.Errorf("pull: %w", err)
	}

	return fmt.Sprintf("on %s — %s", branch, summarisePull(out)), "", nil
}

func branchNotFoundSkip(branch string) string {
	return fmt.Sprintf("branch %q not found (local or remote)", branch)
}

func dirtyCurrentBranchSkip(path, target string) (string, error) {
	current, err := git.CurrentBranch(path)
	if err != nil {
		return "", fmt.Errorf("current branch: %w", err)
	}
	if current != target {
		return "", nil
	}

	dirty, err := git.IsDirtyTrackedOnly(path)
	if err != nil {
		return "", fmt.Errorf("inspect tracked changes: %w", err)
	}
	if !dirty {
		return "", nil
	}

	return fmt.Sprintf("already on %s with uncommitted changes — pull skipped", target), nil
}

func checkoutDefaultDryRunItems(repos []*db.Repository) []dryRunItem {
	return runner.Collect(repos, func(repo *db.Repository) dryRunItem {
		item := dryRunItem{repo: repo}
		skipReason, stateErr := dirtyCurrentBranchSkip(repo.Path, repo.DefaultBranch)
		if skipReason != "" {
			item.skipReason = skipReason
			return item
		}

		item.actions = []string{
			fmt.Sprintf("git checkout %s", repo.DefaultBranch),
			"git pull --ff-only",
		}
		item.warning = "checkout conflicts cannot be predicted without running git checkout"
		if stateErr != nil {
			item.warning = fmt.Sprintf("could not inspect current branch state: %v", stateErr)
		} else if dirty, err := git.IsDirtyTrackedOnly(repo.Path); err == nil && dirty {
			item.warning = "tracked changes exist; checkout may be skipped if Git reports a conflict"
		} else if err != nil {
			item.warning = fmt.Sprintf("could not inspect tracked changes: %v", err)
		}
		return item
	})
}

// checkoutBranchDryRunItems probes origin with ls-remote rather than fetching,
// so a preview never changes the repository.
func checkoutBranchDryRunItems(repos []*db.Repository, branch string) []dryRunItem {
	return runner.Collect(repos, func(repo *db.Repository) dryRunItem {
		item := dryRunItem{repo: repo}
		localExists := git.BranchExists(repo.Path, branch)
		remoteExists := false
		if !localExists {
			remoteExists = git.RemoteBranchExists(repo.Path, branch)
		}
		if !localExists && !remoteExists {
			item.skipReason = branchNotFoundSkip(branch)
			return item
		}

		skipReason, stateErr := dirtyCurrentBranchSkip(repo.Path, branch)
		if skipReason != "" {
			item.skipReason = skipReason
			return item
		}

		if localExists {
			item.actions = []string{
				fmt.Sprintf("git checkout %s", branch),
				"git pull --ff-only",
			}
		} else {
			item.actions = []string{
				fmt.Sprintf("git fetch origin -- %s", branch),
				fmt.Sprintf("git checkout %s", branch),
			}
		}
		item.warning = "checkout conflicts cannot be predicted without running git checkout"
		if stateErr != nil {
			item.warning = fmt.Sprintf("could not inspect current branch state: %v", stateErr)
		} else if dirty, err := git.IsDirtyTrackedOnly(repo.Path); err == nil && dirty {
			item.warning = "tracked changes exist; checkout may be skipped if Git reports a conflict"
		} else if err != nil {
			item.warning = fmt.Sprintf("could not inspect tracked changes: %v", err)
		}
		return item
	})
}

func isCheckoutConflict(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Your local changes") ||
		strings.Contains(msg, "overwritten by checkout") ||
		strings.Contains(msg, "overwritten by merge")
}

func checkoutConflictSkip(repoPath string, checkoutErr error) (bool, string) {
	if !isCheckoutConflict(checkoutErr) {
		return false, ""
	}
	files, filesErr := git.DirtyFiles(repoPath)
	if filesErr != nil {
		return true, "uncommitted changes conflict with target branch"
	}
	reason := fmt.Sprintf("uncommitted changes conflict (%d file(s))", len(files))
	if len(files) > 0 && len(files) <= 3 {
		reason += ": " + strings.Join(files, ", ")
	}
	return true, reason
}

// summarisePull condenses git pull output into a short message.
func summarisePull(out string) string {
	out = strings.TrimSpace(out)
	if strings.Contains(out, "Already up to date") || strings.Contains(out, "Already up-to-date") {
		return "already up to date"
	}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "file") && strings.Contains(l, "changed") {
			return l
		}
	}
	if out != "" {
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				return strings.TrimSpace(lines[i])
			}
		}
	}
	return "pulled"
}

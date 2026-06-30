package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func pushCmd() *cobra.Command {
	var (
		repoAliases []string
		selectAll   bool
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push the current branch of repositories, auto-recovering from a diverged remote",
		Long: `Push the current branch of one or more repositories to origin, in parallel.

Unlike committing, this pushes whatever is already committed locally — useful
when a commit exists but was never pushed (for example after a push was rejected
and you resolved it, or after 'gitm commit --no-push').

Diverged-remote recovery:
  If a push is rejected because the remote branch has advanced (a non-fast-forward
  "fetch first" rejection), gitm automatically fetches origin and rebases your
  local commits on top of it, then retries the push once. This keeps history
  linear and means you do not have to drop into the repository by hand to pull.

  If the rebase hits conflicts, the repository is left in its rebasing state and
  reported so you can resolve the conflicts, run 'git rebase --continue', and
  re-run 'gitm push'.

Repositories on their default branch are not protected: pushing a default branch
that has local commits is a valid operation. Repositories that already track a
remote and have nothing new to push are skipped.

Selection:

  gitm push
      Interactive: pick repositories via the TUI.

  gitm push --repo api-gateway,auth-service
      Push only the named repositories (no prompt).

  gitm push --group backend
      Show only repositories in the backend group when prompting.

  gitm push --all
      Push every registered repository (no prompt).`,
		Example: `  gitm push
  gitm push --all
  gitm push --group backend
  gitm push --repo=api-gateway,auth-service
  gitm push -r api-gateway -g backend`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPushWithUIAndGroup(liveUI{}, selectAll, repoAliases, groupName)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit push to specific repository aliases (comma-separated)")
	cmd.Flags().BoolVarP(&selectAll, "all", "a", false,
		"Push all registered repositories without prompting")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func runPushWithUI(ui ui, selectAll bool, repoAliases []string) error {
	return runPushWithUIAndGroup(ui, selectAll, repoAliases, "")
}

func runPushWithUIAndGroup(ui ui, selectAll bool, repoAliases []string, groupName string) error {
	allRepos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}

	var chosen []*db.Repository
	switch {
	case len(repoAliases) > 0:
		chosen = allRepos
	case selectAll:
		chosen = allRepos
	default:
		chosen, err = ui.MultiSelect(
			allRepos,
			"Select repositories to push",
			false,
			nil,
		)
		if err != nil {
			return err
		}
	}

	if len(chosen) == 0 {
		return nil
	}

	fmt.Printf("\nPushing the current branch of %d repository(ies)…\n\n", len(chosen))

	// Rebase conflicts are recorded here rather than treated as hard failures, so
	// a clear follow-up block can be printed once the parallel run finishes. The
	// mutex guards appends because runner.Run invokes op from many goroutines.
	var (
		mu        sync.Mutex
		conflicts []conflictedRepo
	)

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		out, pushErr := pushRepo(repo)
		if pushErr != nil {
			return "", "", pushErr
		}
		if len(out.conflict) > 0 {
			mu.Lock()
			conflicts = append(conflicts, conflictedRepo{alias: repo.Alias, path: repo.Path, files: out.conflict})
			mu.Unlock()
			return "", fmt.Sprintf("rebase conflict after diverged push — %d file(s) to resolve manually", len(out.conflict)), nil
		}
		if out.skipped {
			return "", out.message, nil
		}
		return out.message, "", nil
	})

	printRebaseConflicts(conflicts)

	// Rebase conflicts are intentional skips, not failures. Only genuine errors
	// (branch lookup, non-conflict push/rebase failures) make the command exit
	// non-zero, matching the other multi-repo commands.
	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to push", runner.ErrorCount(results))
	}
	return nil
}

// pushOutcome describes what pushRepo did for a single repository.
type pushOutcome struct {
	message  string   // human-readable detail for a successful push or a skip
	skipped  bool     // true when there was nothing to push (branch up to date)
	conflict []string // non-empty when the recovery rebase stopped on conflicts
}

// pushRepo pushes repo's current branch to origin, auto-recovering from a
// diverged remote (a rejected non-fast-forward push) by rebasing the local
// commits onto origin and retrying the push once.
//
// A clean push is the common path; the rebase+retry only runs when the first
// push fails. Rebase conflicts are left in place (not aborted) so the user can
// resolve them and re-run, and are surfaced via the conflict field rather than
// as a hard error — mirroring how gitm sync reports merge conflicts.
func pushRepo(repo *db.Repository) (pushOutcome, error) {
	branch, err := git.CurrentBranch(repo.Path)
	if err != nil {
		return pushOutcome{}, fmt.Errorf("get current branch: %w", err)
	}

	// Nothing to push when the branch already tracks a remote and is not ahead.
	// A branch with no upstream is always pushed — the first push creates it.
	if hasUpstream, upErr := git.HasUpstream(repo.Path); upErr == nil && hasUpstream {
		if ahead, _, abErr := git.AheadBehind(repo.Path, false); abErr == nil && ahead == 0 {
			return pushOutcome{skipped: true, message: fmt.Sprintf("nothing to push — %s is up to date", branch)}, nil
		}
	}

	pushErr := git.Push(repo.Path)
	if pushErr == nil {
		return pushOutcome{message: fmt.Sprintf("pushed %s", branch)}, nil
	}

	// The push was rejected — usually because the remote branch advanced. Fetch
	// and rebase our local commits on top of it, then retry the push once.
	if _, rbErr := git.PullRebase(repo.Path, branch); rbErr != nil {
		if unmerged, umErr := git.UnmergedFiles(repo.Path); umErr == nil && len(unmerged) > 0 {
			return pushOutcome{conflict: unmerged}, nil
		}
		return pushOutcome{}, fmt.Errorf("push rejected and auto-rebase failed: %w (original push error: %w)", rbErr, pushErr)
	}

	if retryErr := git.Push(repo.Path); retryErr != nil {
		return pushOutcome{}, fmt.Errorf("rebased onto origin/%s but push still failed: %w", branch, retryErr)
	}
	return pushOutcome{message: fmt.Sprintf("remote had diverged — rebased onto origin/%s and pushed", branch)}, nil
}

// printRebaseConflicts lists repositories whose recovery rebase stopped on
// conflicts, with paths, so the user can resolve each one and finish.
func printRebaseConflicts(conflicts []conflictedRepo) {
	if len(conflicts) == 0 {
		return
	}
	fmt.Printf("\n%d repository(ies) have rebase conflicts left for you to resolve:\n", len(conflicts))
	for _, c := range conflicts {
		fmt.Printf("  - %s (%s)\n", c.alias, c.path)
		for _, f := range c.files {
			fmt.Printf("      conflict: %s\n", f)
		}
	}
	fmt.Println("\nResolve the conflicts in each repo, then `git rebase --continue` and `gitm push` (or `git rebase --abort`).")
}

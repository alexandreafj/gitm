package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func syncCmd() *cobra.Command {
	var (
		repoAliases []string
		selectAll   bool
		dryRun      bool
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "sync [branch]",
		Short: "Merge the latest default branch (main/master) into the current branch",
		Long: `Bring the branch you are working on up to date by merging another branch
into it — across one or many repositories at once.

By default the branch merged in is each repository's default branch (main or
master, auto-detected per repository). Pass a branch name to merge that branch
instead — useful when you track a long-lived integration branch that is not the
repository's configured default:

  gitm sync                merge each repo's default branch (main/master)
  gitm sync master-raw     merge "master-raw" into the current branch

For each selected repository, gitm:
  1. Fetches the latest target branch from origin.
  2. Merges it into whatever branch the repository is currently on.

This replaces the manual, per-repo routine of pulling the latest master/main
and merging it into your working branch with "git merge master" by hand.

Selection:

  gitm sync
      Interactive: pick repositories via the TUI.

  gitm sync --repo api-gateway,auth-service
      Sync only the named repositories (no prompt).

  gitm sync --group backend
      Show only repositories in the backend group when prompting.

  gitm sync --all
      Sync every registered repository (no prompt).

  gitm sync --dry-run
      Preview the fetch and merge commands without fetching, merging, or
      leaving conflicts in any repository.

Repositories are skipped when:
  - they have uncommitted tracked changes (stash or commit first),
  - they are already on the branch being merged (use "gitm update" to pull), or
  - the requested branch does not exist locally or on origin.

Untracked files do not block the sync.

Merge conflicts are left in place so you can resolve them yourself: the repo is
reported and kept in its merging state — resolve the conflicts and commit.`,
		Example: `  gitm sync
  gitm sync --all
  gitm sync master-raw
  gitm sync --group backend
  gitm sync master-raw --group backend
  gitm sync --repo=api-gateway,auth-service
  gitm sync master-raw -r api-gateway -g backend
  gitm sync --all --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := ""
			if len(args) > 0 {
				branch = strings.TrimSpace(args[0])
			}
			return runSyncWithUIAndGroupDryRun(liveUI{}, selectAll, repoAliases, groupName, branch, dryRun)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit sync to specific repository aliases (comma-separated)")
	cmd.Flags().BoolVarP(&selectAll, "all", "a", false,
		"Sync all registered repositories without prompting")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Preview fetch and merge operations without changing repositories")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func runSyncWithUI(ui ui, selectAll bool, repoAliases []string, branch string) error {
	return runSyncWithUIAndGroup(ui, selectAll, repoAliases, "", branch)
}

func runSyncWithUIAndGroup(ui ui, selectAll bool, repoAliases []string, groupName, branch string) error {
	return runSyncWithUIAndGroupDryRun(ui, selectAll, repoAliases, groupName, branch, false)
}

func runSyncWithUIDryRun(ui ui, selectAll bool, repoAliases []string, branch string, dryRun bool) error {
	return runSyncWithUIAndGroupDryRun(ui, selectAll, repoAliases, "", branch, dryRun)
}

func runSyncWithUIAndGroupDryRun(ui ui, selectAll bool, repoAliases []string, groupName, branch string, dryRun bool) error {
	allRepos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}

	// When no branch is given, each repo merges its own configured default
	// branch; an explicit branch overrides that uniformly across repos.
	branchLabel := "default branch"
	selectTitle := "Select repositories to sync with their default branch"
	if branch != "" {
		branchLabel = fmt.Sprintf("%q", branch)
		selectTitle = fmt.Sprintf("Select repositories to sync with %q", branch)
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
			selectTitle,
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

	if dryRun {
		printDryRunPreview(
			fmt.Sprintf("Sync %s preview for %d repository(ies)", branchLabel, len(chosen)),
			syncDryRunItems(chosen, branch),
		)
		return nil
	}

	fmt.Printf("\nMerging %s into the current branch of %d repository(ies)…\n\n", branchLabel, len(chosen))

	// Conflicts are recorded here rather than treated as hard failures, so a
	// clear follow-up block can be printed once the parallel run finishes. The
	// mutex guards appends because runner.Run invokes op from many goroutines.
	var (
		mu        sync.Mutex
		conflicts []conflictedRepo
	)

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("git status: %w", err)
		}
		if dirty {
			return "", "uncommitted changes — stash or commit first", nil
		}

		cur, err := git.CurrentBranch(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("current branch: %w", err)
		}

		target := repo.DefaultBranch
		if branch != "" {
			target = branch
		}
		if cur == target {
			return "", fmt.Sprintf("currently on %q — nothing to merge (use `gitm update` to pull)", target), nil
		}

		// Fetch the latest target branch. A failure (no remote / offline) is not
		// fatal — we fall back to the local branch rather than a stale
		// remote-tracking ref (see mergeRef). The outcome is folded into the
		// result message instead of printed here, so it stays synchronized with
		// the runner's own output.
		fetched := git.FetchBranch(repo.Path, target) == nil

		ref := mergeRef(repo.Path, target, fetched)
		if ref == "" {
			return "", fmt.Sprintf("branch %q not found locally or on origin", target), nil
		}

		out, mergeErr := git.Merge(repo.Path, ref)
		if mergeErr != nil {
			unmerged, unmergedErr := git.UnmergedFiles(repo.Path)
			if unmergedErr == nil && len(unmerged) > 0 {
				mu.Lock()
				conflicts = append(conflicts, conflictedRepo{alias: repo.Alias, path: repo.Path, files: unmerged})
				mu.Unlock()
				return "", fmt.Sprintf("merge conflict — %d file(s) to resolve manually", len(unmerged)), nil
			}
			return "", "", fmt.Errorf("merge %s: %w", ref, mergeErr)
		}

		msg := fmt.Sprintf("merged %s into %s — %s", target, cur, summariseMerge(out))
		if !fetched {
			msg += " (fetch failed; merged without refresh)"
		}
		return msg, "", nil
	})

	printConflicts(conflicts)

	// Merge conflicts are intentional skips, not failures. Only genuine errors
	// (status/branch failures, non-conflict merge errors) make the command exit
	// non-zero, matching the other multi-repo commands.
	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to sync", runner.ErrorCount(results))
	}
	return nil
}

func syncDryRunItems(repos []*db.Repository, branch string) []dryRunItem {
	items := make([]dryRunItem, 0, len(repos))
	for _, repo := range repos {
		item := dryRunItem{repo: repo}

		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
		if err != nil {
			item.skipReason = fmt.Sprintf("git status: %v", err)
			items = append(items, item)
			continue
		}
		if dirty {
			item.skipReason = "uncommitted changes — stash or commit first"
			items = append(items, item)
			continue
		}

		current, err := git.CurrentBranch(repo.Path)
		if err != nil {
			item.skipReason = fmt.Sprintf("current branch: %v", err)
			items = append(items, item)
			continue
		}

		target := repo.DefaultBranch
		if branch != "" {
			target = branch
		}
		if current == target {
			item.skipReason = fmt.Sprintf("currently on %q — nothing to merge (use `gitm update` to pull)", target)
			items = append(items, item)
			continue
		}

		localExists := git.BranchExists(repo.Path, target)
		remoteExists := git.RemoteBranchExists(repo.Path, target)
		switch {
		case remoteExists:
			item.actions = append(item.actions,
				fmt.Sprintf("git fetch origin -- %s", target),
				fmt.Sprintf("git merge --no-edit origin/%s", target),
			)
			item.warning = "merge conflicts cannot be predicted without running git merge"
		case localExists:
			item.actions = append(item.actions, fmt.Sprintf("git merge --no-edit %s", target))
			item.warning = "merge conflicts cannot be predicted without running git merge"
		default:
			item.skipReason = fmt.Sprintf("branch %q not found locally or on origin", target)
		}

		items = append(items, item)
	}
	return items
}

type conflictedRepo struct {
	alias string
	path  string
	files []string
}

// mergeRef returns the ref to merge for the default branch. When the fetch
// succeeded it prefers the freshly-updated remote-tracking origin/<def>;
// otherwise it falls back to the local branch so an offline/failed fetch never
// silently merges a stale remote-tracking ref. As a last resort (no local
// branch) it uses origin/<def> if one exists.
func mergeRef(path, def string, fetched bool) string {
	originRef := "origin/" + def
	if fetched && git.BranchExists(path, originRef) {
		return originRef
	}
	if git.BranchExists(path, def) {
		return def
	}
	if git.BranchExists(path, originRef) {
		return originRef
	}
	return ""
}

// summariseMerge condenses git merge output into a short status string.
func summariseMerge(out string) string {
	out = strings.TrimSpace(out)
	switch {
	case out == "":
		return "merged"
	case strings.Contains(out, "Already up to date"), strings.Contains(out, "Already up-to-date"):
		return "already up to date"
	case strings.Contains(out, "Fast-forward"):
		return "fast-forward"
	default:
		return "merged"
	}
}

// printConflicts lists repositories left in a conflicted state, with paths, so
// the user can resolve each one and commit.
func printConflicts(conflicts []conflictedRepo) {
	if len(conflicts) == 0 {
		return
	}
	fmt.Printf("\n%d repository(ies) have merge conflicts left for you to resolve:\n", len(conflicts))
	for _, c := range conflicts {
		fmt.Printf("  - %s (%s)\n", c.alias, c.path)
		for _, f := range c.files {
			fmt.Printf("      conflict: %s\n", f)
		}
	}
	fmt.Println("\nResolve the conflicts in each repo, then `git add` + `git commit` (or `git merge --abort`).")
}

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
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Merge the latest default branch (main/master) into the current branch",
		Long: `Bring the branch you are working on up to date with the latest default
branch (main or master, auto-detected per repository) by merging it in —
across one or many repositories at once.

For each selected repository, gitm:
  1. Fetches the latest default branch from origin.
  2. Merges it into whatever branch the repository is currently on.

This replaces the manual loop of "gitm checkout master", cd into the repo
folder, and "git merge master" for every repository.

Selection:

  gitm sync
      Interactive: pick repositories via the TUI.

  gitm sync --repo api-gateway,auth-service
      Sync only the named repositories (no prompt).

  gitm sync --all
      Sync every registered repository (no prompt).

Repositories are skipped when:
  - they have uncommitted changes (stash or commit first), or
  - they are already on their default branch (use "gitm update" to pull).

Merge conflicts are left in place so you can resolve them yourself: the repo is
reported and kept in its merging state — resolve the conflicts and commit.`,
		Example: `  gitm sync
  gitm sync --all
  gitm sync --repo=api-gateway,auth-service
  gitm sync -r api-gateway`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncWithUI(liveUI{}, selectAll, repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit sync to specific repository aliases (comma-separated)")
	cmd.Flags().BoolVarP(&selectAll, "all", "a", false,
		"Sync all registered repositories without prompting")

	return cmd
}

func runSyncWithUI(ui ui, selectAll bool, repoAliases []string) error {
	allRepos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
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
			"Select repositories to sync with their default branch",
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

	fmt.Printf("\nMerging default branch into the current branch of %d repository(ies)…\n\n", len(chosen))

	// Conflicts are recorded here rather than treated as hard failures, so a
	// clear follow-up block can be printed once the parallel run finishes. The
	// mutex guards appends because runner.Run invokes op from many goroutines.
	var (
		mu        sync.Mutex
		conflicts []conflictedRepo
	)

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
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

		def := repo.DefaultBranch
		if cur == def {
			return "", fmt.Sprintf("currently on default branch %q — nothing to merge (use `gitm update` to pull)", def), nil
		}

		// Fetch the latest default branch. A failure (no remote / offline) is not
		// fatal — fall back to merging the local default branch.
		if fetchErr := git.FetchBranch(repo.Path, def); fetchErr != nil {
			fmt.Printf("  warning: fetch %s failed on %s: %v\n", def, repo.Alias, fetchErr)
		}

		ref := mergeRef(repo.Path, def)
		if ref == "" {
			return "", fmt.Sprintf("default branch %q not found locally or on origin", def), nil
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

		return fmt.Sprintf("merged %s into %s — %s", def, cur, summariseMerge(out)), "", nil
	})

	printConflicts(conflicts)
	return nil
}

type conflictedRepo struct {
	alias string
	path  string
	files []string
}

// mergeRef returns the most up-to-date ref to merge for the default branch:
// the remote-tracking origin/<def> when it exists, otherwise the local branch.
func mergeRef(path, def string) string {
	if git.BranchExists(path, "origin/"+def) {
		return "origin/" + def
	}
	if git.BranchExists(path, def) {
		return def
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

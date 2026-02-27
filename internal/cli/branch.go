package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
	"github.com/alexandreferreira/gitm/internal/tui"
)

func branchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Create or rename branches across multiple repositories",
	}
	cmd.AddCommand(branchCreateCmd())
	cmd.AddCommand(branchRenameCmd())
	return cmd
}

// ─── branch create ───────────────────────────────────────────────────────────

func branchCreateCmd() *cobra.Command {
	var (
		selectAll  bool
		fromBranch string
	)

	cmd := &cobra.Command{
		Use:   "create <branch-name>",
		Short: "Create a new branch in selected repositories",
		Long: `Interactively select repositories, then create a new branch in each one.
The branch is created from the repository's default branch (main/master)
unless --from is specified. All operations run in parallel.`,
		Example: `  gitm branch create feature/JIRA-123
  gitm branch create feature/JIRA-123 --all
  gitm branch create hotfix/bug --from develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			allRepos, err := database.ListRepositories()
			if err != nil {
				return err
			}
			if len(allRepos) == 0 {
				fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
				return nil
			}

			var chosen []*db.Repository
			if selectAll {
				chosen = allRepos
			} else {
				chosen, err = tui.MultiSelect(
					allRepos,
					fmt.Sprintf("Select repositories for new branch: %s", branchName),
					false,
					nil,
				)
				if err != nil {
					return err
				}
			}

			fmt.Printf("\nCreating branch %q in %d repository(ies)…\n\n", branchName, len(chosen))

			runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
				// Determine base branch.
				base := repo.DefaultBranch
				if fromBranch != "" {
					base = fromBranch
				}

				// Make sure we're on the base branch and up to date.
				dirty, err := git.IsDirty(repo.Path)
				if err != nil {
					return "", "", fmt.Errorf("git status: %w", err)
				}
				if dirty {
					return "", "uncommitted changes — stash or commit first", nil
				}

				if err := git.Checkout(repo.Path, base); err != nil {
					return "", "", fmt.Errorf("checkout %s: %w", base, err)
				}
				if _, err := git.Pull(repo.Path); err != nil {
					// Non-fatal: pull failure doesn't prevent branch creation.
					fmt.Printf("  warning: pull failed on %s: %v\n", repo.Alias, err)
				}

				// Check if the branch already exists.
				if git.BranchExists(repo.Path, branchName) {
					// Just switch to it instead of failing.
					if err := git.Checkout(repo.Path, branchName); err != nil {
						return "", "", fmt.Errorf("checkout existing branch: %w", err)
					}
					return fmt.Sprintf("branch %s already exists — checked out", branchName), "", nil
				}

				if err := git.CreateBranch(repo.Path, branchName); err != nil {
					return "", "", fmt.Errorf("create branch: %w", err)
				}

				return fmt.Sprintf("created %s from %s", branchName, base), "", nil
			})

			return nil
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all registered repositories without prompting")
	cmd.Flags().StringVarP(&fromBranch, "from", "f", "", "Base branch to create from (default: repo's default branch)")

	return cmd
}

// ─── branch rename ───────────────────────────────────────────────────────────

func branchRenameCmd() *cobra.Command {
	var (
		selectAll bool
		noRemote  bool
	)

	cmd := &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a branch locally and on remote across selected repositories",
		Long: `Interactively select repositories, then rename a branch in each one.
Steps per repository:
  1. git branch -m <old> <new>        (local rename)
  2. git push origin --delete <old>   (delete old remote branch)
  3. git push --set-upstream origin <new>  (push new name + set tracking)

Use --no-remote to skip the remote steps.`,
		Example: `  gitm branch rename feature/JIRA-123 feature/JIRA-456
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --all
  gitm branch rename old-name new-name --no-remote`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]

			allRepos, err := database.ListRepositories()
			if err != nil {
				return err
			}
			if len(allRepos) == 0 {
				fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
				return nil
			}

			// Filter to repos that actually have the old branch.
			var reposWithBranch []*db.Repository
			for _, r := range allRepos {
				if git.BranchExists(r.Path, oldName) {
					reposWithBranch = append(reposWithBranch, r)
				}
			}

			if len(reposWithBranch) == 0 {
				return fmt.Errorf("no registered repositories have a branch named %q", oldName)
			}

			var chosen []*db.Repository
			if selectAll {
				chosen = reposWithBranch
			} else {
				chosen, err = tui.MultiSelect(
					reposWithBranch,
					fmt.Sprintf("Select repositories to rename: %s → %s", oldName, newName),
					false,
					nil,
				)
				if err != nil {
					return err
				}
			}

			fmt.Printf("\nRenaming %q → %q in %d repository(ies)…\n\n", oldName, newName, len(chosen))

			runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
				// Local rename.
				if err := git.RenameBranch(repo.Path, oldName, newName); err != nil {
					return "", "", fmt.Errorf("local rename: %w", err)
				}

				if noRemote {
					return fmt.Sprintf("renamed %s → %s (local only)", oldName, newName), "", nil
				}

				// Delete old remote branch (best-effort — may not exist).
				if git.RemoteBranchExists(repo.Path, oldName) {
					if err := git.DeleteRemoteBranch(repo.Path, oldName); err != nil {
						return "", "", fmt.Errorf("delete remote branch %s: %w", oldName, err)
					}
				}

				// Push new branch with upstream tracking.
				if err := git.PushBranch(repo.Path, newName); err != nil {
					return "", "", fmt.Errorf("push %s: %w", newName, err)
				}

				return fmt.Sprintf("renamed %s → %s (local + remote)", oldName, newName), "", nil
			})

			return nil
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all repositories that have the old branch")
	cmd.Flags().BoolVar(&noRemote, "no-remote", false, "Only rename locally, skip remote push")

	return cmd
}

package cli

import (
	"github.com/spf13/cobra"
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

func branchCreateCmd() *cobra.Command {
	var (
		selectAll   bool
		fromBranch  string
		repoAliases []string
	)

	cmd := &cobra.Command{
		Use:   "create <branch-name>",
		Short: "Create a new branch in selected repositories",
		Long: `Interactively select repositories, then create a new branch in each one.
The branch is created from the repository's default branch (main/master)
unless --from is specified. All operations run in parallel.

Use --repo to target specific repositories by alias, bypassing the interactive
selection UI entirely.`,
		Example: `  gitm branch create feature/JIRA-123
  gitm branch create feature/JIRA-123 --all
  gitm branch create feature/JIRA-123 --repo api-gateway,auth-service
  gitm branch create hotfix/bug --from develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBranchCreateWithUI(liveUI{}, args, selectAll, fromBranch, repoAliases)
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all registered repositories without prompting")
	cmd.Flags().StringVarP(&fromBranch, "from", "f", "", "Base branch to create from (default: repo's default branch)")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")

	return cmd
}

func branchRenameCmd() *cobra.Command {
	var (
		selectAll   bool
		noRemote    bool
		repoAliases []string
	)

	cmd := &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a branch locally and on remote across selected repositories",
		Long: `Interactively select repositories, then rename a branch in each one.
Steps per repository:
  1. git branch -m <old> <new>        (local rename)
  2. git push origin --delete <old>   (delete old remote branch)
  3. git push --set-upstream origin <new>  (push new name + set tracking)

Use --no-remote to skip the remote steps.
Use --repo to target specific repositories by alias, bypassing the interactive
selection UI entirely.`,
		Example: `  gitm branch rename feature/JIRA-123 feature/JIRA-456
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --all
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --repo api-gateway,auth-service
  gitm branch rename old-name new-name --no-remote`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBranchRenameWithUI(liveUI{}, args[0], args[1], selectAll, noRemote, repoAliases)
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all repositories that have the old branch")
	cmd.Flags().BoolVar(&noRemote, "no-remote", false, "Only rename locally, skip remote push")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")

	return cmd
}

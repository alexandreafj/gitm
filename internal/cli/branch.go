package cli

import (
	"github.com/spf13/cobra"
)

func branchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Create, rename, or delete branches across multiple repositories",
	}
	cmd.AddCommand(branchCreateCmd())
	cmd.AddCommand(branchRenameCmd())
	cmd.AddCommand(branchDeleteCmd())
	return cmd
}

func branchCreateCmd() *cobra.Command {
	var (
		selectAll   bool
		fromBranch  string
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "create <branch-name>",
		Short: "Create a new branch in selected repositories",
		Long: `Interactively select repositories, then create a new branch in each one.
The branch is created from the repository's default branch (main/master)
unless --from is specified. All operations run in parallel.

Use --repo to target specific repositories by alias, bypassing the interactive
selection UI entirely.
Use --group to limit candidates to repositories in a group.
When both are provided, only matching aliases inside that group are targeted.`,
		Example: `  gitm branch create feature/JIRA-123
  gitm branch create feature/JIRA-123 --all
  gitm branch create feature/JIRA-123 --group backend
  gitm branch create feature/JIRA-123 --repo api-gateway,auth-service
  gitm branch create hotfix/bug --from develop -g backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runBranchCreateWithUI(liveUI{}, args, selectAll, fromBranch, repoAliases)
			}
			return runBranchCreateWithUIAndGroup(liveUI{}, args, selectAll, fromBranch, repoAliases, groupName)
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all registered repositories without prompting")
	cmd.Flags().StringVarP(&fromBranch, "from", "f", "", "Base branch to create from (default: repo's default branch)")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func branchRenameCmd() *cobra.Command {
	var (
		selectAll   bool
		noRemote    bool
		repoAliases []string
		groupName   string
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
selection UI entirely.
Use --group to limit candidates to repositories in a group.
When both are provided, only matching aliases inside that group are targeted.`,
		Example: `  gitm branch rename feature/JIRA-123 feature/JIRA-456
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --all
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --group backend
  gitm branch rename feature/JIRA-123 feature/JIRA-456 --repo api-gateway,auth-service
  gitm branch rename old-name new-name --no-remote -g backend`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runBranchRenameWithUI(liveUI{}, args[0], args[1], selectAll, noRemote, repoAliases)
			}
			return runBranchRenameWithUIAndGroup(liveUI{}, args[0], args[1], selectAll, noRemote, repoAliases, groupName)
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all repositories that have the old branch")
	cmd.Flags().BoolVar(&noRemote, "no-remote", false, "Only rename locally, skip remote push")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func branchDeleteCmd() *cobra.Command {
	var (
		selectAll   bool
		force       bool
		noRemote    bool
		dryRun      bool
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "delete <branch-name>",
		Short: "Delete a branch locally and on remote across selected repositories",
		Long: `Delete a branch in each selected repository — locally and on origin in one
step, so you never have to run "git branch -d" and "git push origin --delete"
by hand.

Per repository:
  1. git branch -d <branch-name>          (local delete; -D when --force)
  2. git push origin --delete <branch-name>  (delete the remote branch)

Safety:
  - The local delete uses "git branch -d", which refuses branches with
    unmerged commits. Pass --force to delete them anyway ("git branch -D").
  - The repository's default branch (main/master) is never deleted.
  - A branch that is currently checked out is skipped — switch away first.

Use --no-remote to delete only the local branch.
Use --dry-run to preview exactly which local and remote delete commands would
run without deleting anything or asking for confirmation.
Use --repo to target specific repositories by alias, bypassing the interactive
selection UI. Non-interactive runs (--all or --repo) ask for confirmation
before deleting.
Use --group to limit candidates to repositories in a group.
When both are provided, only matching aliases inside that group are targeted.`,
		Example: `  gitm branch delete feature/JIRA-123
  gitm branch delete feature/JIRA-123 --all
  gitm branch delete feature/JIRA-123 --group backend
  gitm branch delete feature/JIRA-123 --repo api-gateway,auth-service
  gitm branch delete feature/JIRA-123 --force
  gitm branch delete feature/JIRA-123 --no-remote -g backend
  gitm branch delete feature/JIRA-123 --all --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runBranchDeleteWithUIDryRun(liveUI{}, args[0], selectAll, force, noRemote, repoAliases, dryRun)
			}
			return runBranchDeleteWithUIAndGroupDryRun(liveUI{}, args[0], selectAll, force, noRemote, repoAliases, groupName, dryRun)
		},
	}

	cmd.Flags().BoolVarP(&selectAll, "all", "a", false, "Apply to all repositories that have the branch")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force-delete branches with unmerged commits (git branch -D)")
	cmd.Flags().BoolVar(&noRemote, "no-remote", false, "Only delete locally, skip the remote branch")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the branches that would be deleted without changing anything")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")
	addGroupFlag(cmd, &groupName)

	return cmd
}

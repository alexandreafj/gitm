package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
)

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage registered repositories",
		Long:  "Add, list, remove, and rename repositories tracked by gitm.",
	}

	cmd.AddCommand(repoAddCmd())
	cmd.AddCommand(repoListCmd())
	cmd.AddCommand(repoRemoveCmd())
	cmd.AddCommand(repoRenameCmd())

	return cmd
}

// repoAddCmd adds one or more repositories.
func repoAddCmd() *cobra.Command {
	var alias string

	cmd := &cobra.Command{
		Use:   "add <path> [path...]",
		Short: "Register one or more git repositories",
		Example: `  gitm repo add .
  gitm repo add /home/user/work/api-gateway
  gitm repo add /home/user/work/api-gateway /home/user/work/auth-service
  gitm repo add /home/user/work/www-api/v1 --alias www-v1`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if alias != "" && len(args) > 1 {
				return fmt.Errorf("--alias can only be used when adding a single repository")
			}

			var added, failed int
			for _, arg := range args {
				abs, err := filepath.Abs(arg)
				if err != nil {
					color.Red("  ✗ %s: cannot resolve path: %v", arg, err)
					failed++
					continue
				}

				if !git.IsGitRepo(abs) {
					color.Red("  ✗ %s: not a git repository", abs)
					failed++
					continue
				}

				name := git.RepoName(abs)
				displayAlias := alias
				if displayAlias == "" {
					displayAlias = name
				}

				defaultBranch, err := git.DefaultBranch(abs)
				if err != nil {
					defaultBranch = "main"
				}

				_, err = database.AddRepository(name, displayAlias, abs, defaultBranch)
				if err != nil {
					if strings.Contains(err.Error(), "UNIQUE constraint") {
						// Check alias collision first.
						aliasOwner, aliasErr := database.GetRepository(displayAlias)
						if aliasErr == nil && aliasOwner.Path != abs {
							color.Red("  ✗ alias %q is already used by %s", displayAlias, aliasOwner.Path)
							fmt.Printf("     Use --alias to give this repo a unique name, e.g.:\n")
							fmt.Printf("       gitm repo add %s --alias <your-alias>\n", abs)
						} else if existing, pathErr := database.GetRepositoryByPath(abs); pathErr == nil {
							// Path already registered under a (possibly different) alias.
							color.Yellow("  ⚠ %s: already registered as %q", abs, existing.Alias)
						} else {
							color.Yellow("  ⚠ %s: already registered", displayAlias)
						}
					} else {
						color.Red("  ✗ %s: %v", displayAlias, err)
						failed++
					}
					continue
				}

				color.Green("  ✓ added %s (default branch: %s)", displayAlias, defaultBranch)
				added++
			}

			if added > 0 {
				fmt.Printf("\n%d repository(ies) registered. Run `gitm repo list` to see all.\n", added)
			}
			if failed > 0 {
				return fmt.Errorf("%d path(s) could not be added", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&alias, "alias", "", "Custom display name for the repository (must be unique)")
	return cmd
}

// repoListCmd lists all registered repositories.
func repoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repos, err := database.ListRepositories()
			if err != nil {
				return err
			}
			if len(repos) == 0 {
				fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
				return nil
			}

			printRepoTable(repos)
			return nil
		},
	}
}

// repoRemoveCmd removes a repository by alias.
func repoRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <alias>",
		Aliases: []string{"rm"},
		Short:   "Unregister a repository by alias",
		Example: `  gitm repo remove api-gateway
  gitm repo rm www-v1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			if err := database.RemoveRepository(alias); err != nil {
				if err == db.ErrNotFound {
					return fmt.Errorf("repository %q not found — run `gitm repo list` to see registered repos", alias)
				}
				return err
			}
			color.Green("  ✓ removed %s", alias)
			return nil
		},
	}
}

// repoRenameCmd renames an existing repository alias.
func repoRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-alias> <new-alias>",
		Short: "Rename a registered repository's alias",
		Example: `  gitm repo rename v1 www-v1
  gitm repo rename v2 www-v2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldAlias, newAlias := args[0], args[1]
			if err := database.RenameRepository(oldAlias, newAlias); err != nil {
				if err == db.ErrNotFound {
					return fmt.Errorf("repository %q not found — run `gitm repo list` to see registered repos", oldAlias)
				}
				if strings.Contains(err.Error(), "UNIQUE constraint") {
					return fmt.Errorf("alias %q is already in use — choose a different name", newAlias)
				}
				return err
			}
			color.Green("  ✓ renamed %s → %s", oldAlias, newAlias)
			return nil
		},
	}
}

// printRepoTable renders a clean table of repositories.
func printRepoTable(repos []*db.Repository) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgWhite)

	fmt.Printf("%-4s  %-24s  %-14s  %s\n",
		header.Sprint("#"),
		header.Sprint("ALIAS"),
		header.Sprint("DEFAULT BRANCH"),
		header.Sprint("PATH"),
	)

	for i, r := range repos {
		fmt.Printf("%-4d  %-24s  %-14s  %s\n",
			i+1,
			cyan.Sprint(r.Alias),
			dim.Sprint(r.DefaultBranch),
			dim.Sprint(r.Path),
		)
	}

	fmt.Printf("\n%d repository(ies) registered.\n", len(repos))
}

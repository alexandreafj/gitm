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
		Long:  "Add, list, and remove repositories tracked by gitm.",
	}

	cmd.AddCommand(repoAddCmd())
	cmd.AddCommand(repoListCmd())
	cmd.AddCommand(repoRemoveCmd())

	return cmd
}

// repoAddCmd adds one or more repositories.
func repoAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path> [path...]",
		Short: "Register one or more git repositories",
		Example: `  gitm repo add .
  gitm repo add /home/user/work/api-gateway
  gitm repo add /home/user/work/api-gateway /home/user/work/auth-service`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				defaultBranch, err := git.DefaultBranch(abs)
				if err != nil {
					defaultBranch = "main"
				}

				_, err = database.AddRepository(name, abs, defaultBranch)
				if err != nil {
					if strings.Contains(err.Error(), "UNIQUE constraint") {
						color.Yellow("  ⚠ %s: already registered", name)
					} else {
						color.Red("  ✗ %s: %v", name, err)
						failed++
					}
					continue
				}

				color.Green("  ✓ added %s (default branch: %s)", name, defaultBranch)
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

// repoRemoveCmd removes a repository by name.
func repoRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Unregister a repository by name",
		Example: `  gitm repo remove api-gateway
  gitm repo rm auth-service`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := database.RemoveRepository(name); err != nil {
				if err == db.ErrNotFound {
					return fmt.Errorf("repository %q not found — run `gitm repo list` to see registered repos", name)
				}
				return err
			}
			color.Green("  ✓ removed %s", name)
			return nil
		},
	}
}

// printRepoTable renders a clean table of repositories.
func printRepoTable(repos []*db.Repository) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgWhite)

	fmt.Printf("%-4s  %-22s  %-14s  %s\n",
		header.Sprint("#"),
		header.Sprint("NAME"),
		header.Sprint("DEFAULT BRANCH"),
		header.Sprint("PATH"),
	)

	for i, r := range repos {
		fmt.Printf("%-4d  %-22s  %-14s  %s\n",
			i+1,
			cyan.Sprint(r.Name),
			dim.Sprint(r.DefaultBranch),
			dim.Sprint(r.Path),
		)
	}

	fmt.Printf("\n%d repository(ies) registered.\n", len(repos))
}

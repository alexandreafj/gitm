package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
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
	var autoDetect bool
	var depth int

	cmd := &cobra.Command{
		Use:   "add <path> [path...]",
		Short: "Register one or more git repositories",
		Long: `Register one or more local git repositories with gitm.

Paths can be absolute or relative. Use "." for the current directory.

With --auto-detect, provide a single parent directory and gitm will scan its
subdirectories, registering every git repository it finds. By default it scans
one level deep (immediate children); use --depth N to scan N levels deep. This
is useful when repos are nested inside grouping folders (e.g. project/v1, project/v2).`,
		Example: `  gitm repo add .
  gitm repo add /home/user/work/api-gateway
  gitm repo add /home/user/work/api-gateway /home/user/work/auth-service
  gitm repo add /home/user/work/www-api/v1 --alias www-v1

  # Scan a folder and register every git repo found inside it
  gitm repo add /home/user/work --auto-detect

  # Scan two levels deep to find repos in subfolders
  gitm repo add /home/user/work --auto-detect --depth 2`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if alias != "" && len(args) > 1 {
				return fmt.Errorf("--alias can only be used when adding a single repository")
			}
			if autoDetect && alias != "" {
				return fmt.Errorf("--auto-detect and --alias cannot be used together")
			}
			if autoDetect && len(args) > 1 {
				return fmt.Errorf("--auto-detect requires exactly one path argument (the parent directory to scan)")
			}
			if !autoDetect && cmd.Flags().Changed("depth") {
				return fmt.Errorf("--depth can only be used with --auto-detect")
			}
			if depth < 1 {
				return fmt.Errorf("--depth must be at least 1")
			}

			// --auto-detect: expand the single parent dir into its git-repo children.
			paths := args
			if autoDetect {
				discovered, err := discoverRepos(args[0], depth)
				if err != nil {
					return err
				}
				if len(discovered) == 0 {
					fmt.Println("No git repositories found in the specified directory.")
					return nil
				}
				fmt.Printf("Found %d git repository(ies) in %s\n\n", len(discovered), args[0])
				paths = discovered
			}

			var added, failed int
			for _, arg := range paths {
				abs, err := filepath.Abs(arg)
				if err != nil {
					color.Red("  ✗ %s: cannot resolve path: %v", arg, err)
					failed++
					continue
				}

				// Resolve symlinks so the canonical path is persisted.
				// This prevents registering the same repo twice under
				// different-but-equivalent paths (e.g. /var/... vs
				// /private/var/... on macOS).
				if resolved, err := filepath.EvalSymlinks(abs); err == nil {
					abs = resolved
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
						// Check if this is a path duplicate (idempotent re-add).
						if existing, pathErr := database.GetRepositoryByPath(abs); pathErr == nil {
							// Path already registered under a (possibly different) alias.
							color.Yellow("  ⚠ %s: already registered as %q", abs, existing.Alias)
						} else if aliasOwner, aliasErr := database.GetRepository(displayAlias); aliasErr == nil {
							// Alias collision: the alias is taken by a different repo.
							// Resolve the stored path to handle symlink differences.
							ownerPath := aliasOwner.Path
							if resolved, evalErr := filepath.EvalSymlinks(ownerPath); evalErr == nil {
								ownerPath = resolved
							}
							if ownerPath != abs {
								color.Red("  ✗ alias %q is already used by %s", displayAlias, aliasOwner.Path)
								fmt.Printf("     Use --alias to give this repo a unique name, e.g.:\n")
								fmt.Printf("       gitm repo add %s --alias <your-alias>\n", abs)
								failed++
							} else {
								color.Yellow("  ⚠ %s: already registered", displayAlias)
							}
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
	cmd.Flags().BoolVar(&autoDetect, "auto-detect", false, "Scan subdirectories of the given path and register every git repository found (default depth 1; use --depth to scan deeper)")
	cmd.Flags().IntVar(&depth, "depth", 1, "How many directory levels to scan when using --auto-detect (default 1)")
	return cmd
}

// discoverRepos scans subdirectories of parentDir up to maxDepth levels deep
// and returns the absolute paths of every directory that is the root of a git
// repository. Hidden directories (those whose name starts with ".") are skipped.
// When a git repo is found, its children are not scanned (the repo is a leaf).
func discoverRepos(parentDir string, maxDepth int) ([]string, error) {
	abs, err := filepath.Abs(parentDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %q: %w", parentDir, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot access %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", abs)
	}

	var repos []string
	visited := make(map[string]bool)

	var walk func(dir string, currentDepth int) error
	walk = func(dir string, currentDepth int) error {
		if currentDepth > maxDepth {
			return nil
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("cannot read directory %q: %w", dir, err)
		}

		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			candidate := filepath.Join(dir, entry.Name())

			// Use os.Stat (not entry.IsDir) so that symlinks pointing to
			// directories are followed.
			info, err := os.Stat(candidate)
			if err != nil || !info.IsDir() {
				continue
			}

			realPath, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				continue
			}
			if visited[realPath] {
				continue
			}
			visited[realPath] = true

			if git.IsGitRepo(candidate) {
				repos = append(repos, candidate)
				continue
			}

			if currentDepth < maxDepth {
				if err := walk(candidate, currentDepth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(abs, 1); err != nil {
		return nil, err
	}
	return repos, nil
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
				if errors.Is(err, db.ErrNotFound) {
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
				if errors.Is(err, db.ErrNotFound) {
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

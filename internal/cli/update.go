package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func updateCmd() *cobra.Command {
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull latest changes on the current branch of all repositories",
		Long: `Run git pull on the current branch of every registered repository in parallel.
Unlike 'checkout master', this does NOT switch branches — it just pulls
whatever branch each repo is currently on.

Repositories with uncommitted changes are skipped.

Use --repo to limit the update to specific repositories by alias.
The flag can be repeated to target multiple repos.`,
		Example: `  gitm update
  gitm update --repo=api-gateway
  gitm update --repo=api-gateway,auth-service,frontend
  gitm update -r api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit update to specific repository aliases (comma-separated)")

	return cmd
}

func runUpdate(repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	fmt.Printf("Pulling current branch for %d repositories…\n\n", len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("git status: %w", err)
		}
		if dirty {
			return "", "uncommitted changes — stash or commit first", nil
		}

		branch, err := git.CurrentBranch(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("get current branch: %w", err)
		}

		out, err := git.Pull(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("pull: %w", err)
		}

		msg := fmt.Sprintf("on %s — %s", branch, summarisePull(out))
		return msg, "", nil
	})

	return nil
}

func resolveRepos(aliases []string) ([]*db.Repository, error) {
	if len(aliases) == 0 {
		return database.ListRepositories()
	}

	seen := make(map[string]bool, len(aliases))
	repos := make([]*db.Repository, 0, len(aliases))
	for _, alias := range aliases {
		if seen[alias] {
			continue
		}
		seen[alias] = true

		repo, err := database.GetRepository(alias)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return nil, fmt.Errorf("repository %q not found — run `gitm repo list` to see registered repos", alias)
			}
			return nil, fmt.Errorf("lookup %q: %w", alias, err)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

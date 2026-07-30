package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func updateCmd() *cobra.Command {
	var (
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull latest changes on the current branch of all repositories",
		Long: `Run git pull on the current branch of every registered repository in parallel.
Unlike 'checkout master', this normally keeps each repository on its current
branch.

If the remote branch no longer exists (e.g. deleted after a PR merge), the
repository is automatically switched to its default branch and pulled. Only in
that fallback path, gitm performs a lightweight network lookup of origin's
symbolic HEAD (not a full fetch) and updates GitM's SQLite cache when it changed.
If the lookup fails, gitm warns and uses the cached default. Ordinary successful
updates do not refresh the default-branch cache.

Repositories with uncommitted changes are skipped.
Branches with no upstream are skipped — there is nothing to pull until the
branch is pushed (gitm push sets the upstream).

Use --repo to limit the update to specific repositories by alias.
Use --group to limit the update to repositories in a group.
When both are provided, gitm updates only aliases that also belong to the group.
The repo flag can be repeated to target multiple repos.`,
		Example: `  gitm update
  gitm update --repo=api-gateway
  gitm update --repo=api-gateway,auth-service,frontend
  gitm update -r api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runUpdate(repoAliases)
			}
			return runUpdateWithGroup(repoAliases, groupName)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil,
		"Limit update to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &groupName)

	return cmd
}

func runUpdate(repoAliases []string) error {
	return runUpdateWithGroup(repoAliases, "")
}

func runUpdateWithGroup(repoAliases []string, groupName string) error {
	return runUpdateWithGroupTo(repoAliases, groupName, os.Stdout)
}

func runUpdateWithGroupTo(repoAliases []string, groupName string, output io.Writer) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Fprintln(output, noReposMessage(repoAliases, groupName))
		return nil
	}

	fmt.Fprintf(output, "Pulling current branch for %d repositories…\n\n", len(repos))

	type updatePreparation struct {
		branch        string
		message       string
		skipReason    string
		err           error
		needsFallback bool
	}

	prepared := make([]updatePreparation, len(repos))
	indexes := make(map[*db.Repository]int, len(repos))
	for i, repo := range repos {
		indexes[repo] = i
	}
	results := runner.RunEach(repos, func(repo *db.Repository) (string, string, error) {
		i := indexes[repo]
		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
		if err != nil {
			prepared[i].err = fmt.Errorf("git status: %w", err)
			return "", "", prepared[i].err
		}
		if dirty {
			prepared[i].skipReason = "uncommitted changes — stash or commit first"
			return "", prepared[i].skipReason, nil
		}

		branch, err := git.CurrentBranch(repo.Path)
		if err != nil {
			prepared[i].err = fmt.Errorf("get current branch: %w", err)
			return "", "", prepared[i].err
		}
		prepared[i].branch = branch

		out, pullErr := git.Pull(repo.Path)
		switch {
		case pullErr == nil:
			prepared[i].message = fmt.Sprintf("on %s — %s", branch, summarisePull(out))
		case git.IsNoUpstreamError(pullErr):
			prepared[i].skipReason = fmt.Sprintf("on %s — no upstream, nothing to pull (`gitm push` sets one)", branch)
		case strings.Contains(pullErr.Error(), "no such ref was fetched"):
			prepared[i].needsFallback = true
		default:
			prepared[i].err = fmt.Errorf("pull: %w", pullErr)
		}
		return prepared[i].message, prepared[i].skipReason, prepared[i].err
	}, func(result runner.Result) {
		if !prepared[indexes[result.Repo]].needsFallback {
			runner.FprintResult(output, result)
		}
	})

	fallbackRepos := make([]*db.Repository, 0)
	for i, repo := range repos {
		if prepared[i].needsFallback {
			fallbackRepos = append(fallbackRepos, repo)
		}
	}
	if err := reconcileDefaultBranches(database, fallbackRepos, true); err != nil {
		return fmt.Errorf("refresh default branches for update fallback: %w", err)
	}

	fallbackResults := runner.RunEach(fallbackRepos, func(repo *db.Repository) (string, string, error) {
		prep := &prepared[indexes[repo]]
		def := repo.DefaultBranch
		if err := git.Checkout(repo.Path, def); err != nil {
			return "", "", fmt.Errorf("remote branch gone, switch to %s failed: %w", def, err)
		}
		out, err := git.Pull(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("pull %s: %w", def, err)
		}
		return fmt.Sprintf("remote branch %s gone → switched to %s — %s", prep.branch, def, summarisePull(out)), "", nil
	}, func(result runner.Result) {
		runner.FprintResult(output, result)
	})
	for i, repo := range fallbackRepos {
		results[indexes[repo]] = fallbackResults[i]
	}
	runner.FprintSummary(output, results)

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to update", runner.ErrorCount(results))
	}
	return nil
}

func resolveRepos(aliases []string) ([]*db.Repository, error) {
	return resolveReposByAlias(aliases)
}

// noReposMessage returns the right empty-state message: a registration hint when
// nothing is registered at all, or a filter hint when --repo/--group narrowed the
// set to nothing (so we don't wrongly tell the user to add repositories).
func noReposMessage(aliases []string, groupName string) string {
	if len(aliases) > 0 || strings.TrimSpace(groupName) != "" {
		return "No repositories match the given --repo/--group filters."
	}
	return "No repositories registered. Run `gitm repo add <path>` to add one."
}

func resolveReposWithGroup(aliases []string, groupName string) ([]*db.Repository, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return resolveRepos(aliases)
	}

	groupRepos, err := database.ListRepositoriesByGroup(groupName)
	if err != nil {
		if errors.Is(err, db.ErrGroupNotFound) {
			return nil, fmt.Errorf("group %q not found — run `gitm group list` to see groups: %w", groupName, err)
		}
		return nil, fmt.Errorf("list repositories in group %q: %w", groupName, err)
	}
	if len(aliases) == 0 {
		return groupRepos, nil
	}

	groupAliases := make(map[string]bool, len(groupRepos))
	for _, repo := range groupRepos {
		groupAliases[repo.Alias] = true
	}

	repos, err := resolveReposByAlias(aliases)
	if err != nil {
		return nil, err
	}
	filtered := make([]*db.Repository, 0, len(repos))
	for _, repo := range repos {
		if groupAliases[repo.Alias] {
			filtered = append(filtered, repo)
		}
	}
	return filtered, nil
}

func resolveReposByAlias(aliases []string) ([]*db.Repository, error) {
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

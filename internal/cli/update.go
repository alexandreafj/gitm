package cli

import (
	"errors"
	"fmt"
	"strings"
	"sync"

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
Unlike 'checkout master', this does NOT switch branches — it just pulls
whatever branch each repo is currently on.

If the remote branch no longer exists (e.g. deleted after a PR merge), the
repository is automatically switched to its default branch and pulled.

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
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}

	fmt.Printf("Pulling current branch for %d repositories…\n\n", len(repos))

	type updatePreparation struct {
		branch        string
		message       string
		skipReason    string
		err           error
		needsFallback bool
	}

	prepared := make([]updatePreparation, len(repos))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dirty, err := git.IsDirtyTrackedOnly(repo.Path)
			if err != nil {
				prepared[i].err = fmt.Errorf("git status: %w", err)
				return
			}
			if dirty {
				prepared[i].skipReason = "uncommitted changes — stash or commit first"
				return
			}

			branch, err := git.CurrentBranch(repo.Path)
			if err != nil {
				prepared[i].err = fmt.Errorf("get current branch: %w", err)
				return
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
		}()
	}
	wg.Wait()

	fallbackRepos := make([]*db.Repository, 0)
	preparedByRepo := make(map[*db.Repository]*updatePreparation, len(repos))
	for i, repo := range repos {
		preparedByRepo[repo] = &prepared[i]
		if prepared[i].needsFallback {
			fallbackRepos = append(fallbackRepos, repo)
		}
	}
	if err := reconcileDefaultBranches(database, fallbackRepos, true); err != nil {
		return fmt.Errorf("refresh default branches for update fallback: %w", err)
	}

	results := runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		prep := preparedByRepo[repo]
		if !prep.needsFallback {
			return prep.message, prep.skipReason, prep.err
		}

		def := repo.DefaultBranch
		if err := git.Checkout(repo.Path, def); err != nil {
			return "", "", fmt.Errorf("remote branch gone, switch to %s failed: %w", def, err)
		}
		out, err := git.Pull(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("pull %s: %w", def, err)
		}
		return fmt.Sprintf("remote branch %s gone → switched to %s — %s", prep.branch, def, summarisePull(out)), "", nil
	})

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

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

// reconcileDefaultBranches confirms each repo's cached default branch against
// its local origin/HEAD, asking origin only when that ref is missing, dangling,
// or different from the cache. Git never updates an existing origin/HEAD, so a
// disagreement is settled by origin rather than by the local ref.
func reconcileDefaultBranches(database *db.DB, repos []*db.Repository, persist bool) error {
	return reconcileDefaultBranchesWith(database, repos, persist, func(repo *db.Repository) (string, error) {
		local, err := git.LocalDefaultBranch(repo.Path)
		if err != nil {
			return "", err
		}
		if local != "" && local == repo.DefaultBranch {
			return local, nil
		}
		return liveDefaultBranch(repo.Path, local, persist)
	})
}

// liveDefaultBranch asks origin for its default branch. With record set it also
// points origin/HEAD at the answer when it differs from local, so later local
// lookups agree with it; dry runs pass false to leave Git metadata untouched.
func liveDefaultBranch(path, local string, record bool) (string, error) {
	branch, err := git.RemoteDefaultBranch(path)
	if err != nil || !record || branch == local {
		return branch, err
	}
	if err := git.SetRemoteHead(path, branch); err != nil {
		return "", err
	}
	return branch, nil
}

func reconcileDefaultBranchesWith(database *db.DB, repos []*db.Repository, persist bool, lookup func(repo *db.Repository) (string, error)) error {
	type lookupResult struct {
		branch string
		err    error
	}

	results := runner.Collect(repos, func(repo *db.Repository) lookupResult {
		branch, err := lookup(repo)
		return lookupResult{branch: branch, err: err}
	})

	failedAliases := make([]string, 0)
	changed := make([]*db.Repository, 0)
	for i, result := range results {
		if result.err != nil {
			failedAliases = append(failedAliases, repos[i].Alias)
			continue
		}
		if result.branch == repos[i].DefaultBranch {
			continue
		}
		repos[i].DefaultBranch = result.branch
		changed = append(changed, repos[i])
	}

	if len(failedAliases) > 0 {
		sort.Strings(failedAliases)
		fmt.Printf("Warning: could not refresh default branch for %s; using cached values.\n", strings.Join(failedAliases, ", "))
	}

	if !persist {
		return nil
	}
	for _, repo := range changed {
		if err := database.UpdateDefaultBranch(repo.Alias, repo.DefaultBranch); err != nil {
			return fmt.Errorf("persist default branch for %s: %w", repo.Alias, err)
		}
	}
	return nil
}

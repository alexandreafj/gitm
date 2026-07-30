package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func reconcileDefaultBranches(database *db.DB, repos []*db.Repository, persist bool) error {
	type lookupResult struct {
		branch string
		err    error
	}

	results := make([]lookupResult, len(repos))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[i].branch, results[i].err = git.RemoteDefaultBranch(repo.Path)
		}()
	}
	wg.Wait()

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

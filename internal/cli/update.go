package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
)

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Pull latest changes on the current branch of all repositories",
		Long: `Run git pull on the current branch of every registered repository in parallel.
Unlike 'checkout master', this does NOT switch branches — it just pulls
whatever branch each repo is currently on.

Repositories with uncommitted changes are skipped.`,
		Args: cobra.NoArgs,
		RunE: runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	fmt.Printf("Pulling current branch for %d repositories…\n\n", len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		// Skip repos with tracked modifications (untracked files are safe to ignore).
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

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
)

func checkoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Checkout and pull the default branch across all repositories",
		Long: `Switch every registered repository to its default branch (main or master)
and pull the latest changes. Repositories with uncommitted changes are
skipped with a warning — nothing is ever force-reset.`,
		Example: `  gitm checkout master`,
		Args:    cobra.NoArgs,
		RunE:    runCheckout,
	}
	return cmd
}

func runCheckout(cmd *cobra.Command, args []string) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	fmt.Printf("Checking out default branch and pulling for %d repositories…\n\n", len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		// Check for uncommitted changes first.
		dirty, err := git.IsDirty(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("git status failed: %w", err)
		}
		if dirty {
			files, _ := git.DirtyFiles(repo.Path)
			reason := fmt.Sprintf("uncommitted changes (%d file(s))", len(files))
			if len(files) > 0 && len(files) <= 3 {
				reason += ": " + strings.Join(files, ", ")
			}
			return "", reason, nil
		}

		// Checkout the default branch.
		if err := git.Checkout(repo.Path, repo.DefaultBranch); err != nil {
			return "", "", fmt.Errorf("checkout %s: %w", repo.DefaultBranch, err)
		}

		// Pull latest.
		out, err := git.Pull(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("pull: %w", err)
		}

		msg := fmt.Sprintf("on %s — %s", repo.DefaultBranch, summarisePull(out))
		return msg, "", nil
	})

	return nil
}

// summarisePull condenses git pull output into a short message.
func summarisePull(out string) string {
	out = strings.TrimSpace(out)
	if strings.Contains(out, "Already up to date") || strings.Contains(out, "Already up-to-date") {
		return "already up to date"
	}
	// Extract "n file(s) changed" from the pull output if present.
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "file") && strings.Contains(l, "changed") {
			return l
		}
	}
	if out != "" {
		// Return the last non-empty line as a short summary.
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				return strings.TrimSpace(lines[i])
			}
		}
	}
	return "pulled"
}

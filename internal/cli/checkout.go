package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
	"github.com/alexandreferreira/gitm/internal/tui"
)

// defaultBranchKeywords are the argument values that trigger "checkout default branch" mode.
var defaultBranchKeywords = map[string]bool{
	"master": true,
	"main":   true,
}

func checkoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout [branch]",
		Short: "Checkout a branch across repositories",
		Long: `Switch repositories to a branch and pull the latest changes.

Three modes of operation:

  gitm checkout
      Interactive: select repos via TUI, then type a branch name.
      Skips repos where the branch doesn't exist.

  gitm checkout master  (or: gitm checkout main)
      Switches ALL repos to their configured default branch and pulls.

  gitm checkout <branch-name>
      Checks out <branch-name> in ALL repos where it exists.
      Repos where the branch is not found are skipped with a warning.

Repositories with uncommitted tracked changes are always skipped.`,
		Example: `  gitm checkout
  gitm checkout master
  gitm checkout feature/JIRA-12345`,
		Args: cobra.ArbitraryArgs,
		RunE: runCheckout,
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

	// Determine mode from args.
	arg := ""
	if len(args) > 0 {
		arg = strings.TrimSpace(args[0])
	}

	switch {
	case arg == "":
		// Interactive mode.
		return runCheckoutInteractive(repos)

	case defaultBranchKeywords[strings.ToLower(arg)]:
		// Default branch mode.
		return runCheckoutDefault(repos)

	default:
		// Specific branch mode.
		return runCheckoutBranch(repos, arg)
	}
}

// runCheckoutDefault switches all repos to their configured default branch and pulls.
func runCheckoutDefault(repos []*db.Repository) error {
	fmt.Printf("Checking out default branch and pulling for %d repositories…\n\n", len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
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

		if checkoutErr := git.Checkout(repo.Path, repo.DefaultBranch); checkoutErr != nil {
			return "", "", fmt.Errorf("checkout %s: %w", repo.DefaultBranch, checkoutErr)
		}

		out, err := git.Pull(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("pull: %w", err)
		}

		return fmt.Sprintf("on %s — %s", repo.DefaultBranch, summarisePull(out)), "", nil
	})

	return nil
}

// runCheckoutBranch checks out a specific branch in all repos, skipping those
// where the branch does not exist locally or remotely.
func runCheckoutBranch(repos []*db.Repository, branch string) error {
	fmt.Printf("Checking out branch %q in %d repositories…\n\n", branch, len(repos))

	runner.Run(repos, func(repo *db.Repository) (string, string, error) {
		return checkoutBranchInRepo(repo, branch)
	})

	return nil
}

// runCheckoutInteractive lets the user pick repos via TUI, type a branch name,
// then checks out that branch in the selected repos.
func runCheckoutInteractive(repos []*db.Repository) error {
	chosen, err := tui.MultiSelect(repos, "Select repositories to checkout", false, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	branch, err := tui.BranchNameInput()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("\nChecking out branch %q in %d repositories…\n\n", branch, len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		return checkoutBranchInRepo(repo, branch)
	})

	return nil
}

// checkoutBranchInRepo performs the dirty check, branch-existence check,
// checkout, and pull for a single repo. Shared by both specific and interactive modes.
func checkoutBranchInRepo(repo *db.Repository, branch string) (string, string, error) {
	// Skip if tracked files are dirty.
	dirty, err := git.IsDirtyTrackedOnly(repo.Path)
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

	// Check branch existence: local first, then remote.
	localExists := git.BranchExists(repo.Path, branch)
	remoteExists := false
	if !localExists {
		remoteExists = git.RemoteBranchExists(repo.Path, branch)
	}

	if !localExists && !remoteExists {
		return "", fmt.Sprintf("branch %q not found (local or remote)", branch), nil
	}

	if checkoutErr := git.Checkout(repo.Path, branch); checkoutErr != nil {
		return "", "", fmt.Errorf("checkout: %w", checkoutErr)
	}

	out, err := git.Pull(repo.Path)
	if err != nil {
		return "", "", fmt.Errorf("pull: %w", err)
	}

	return fmt.Sprintf("on %s — %s", branch, summarisePull(out)), "", nil
}

// summarisePull condenses git pull output into a short message.
func summarisePull(out string) string {
	out = strings.TrimSpace(out)
	if strings.Contains(out, "Already up to date") || strings.Contains(out, "Already up-to-date") {
		return "already up to date"
	}
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "file") && strings.Contains(l, "changed") {
			return l
		}
	}
	if out != "" {
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				return strings.TrimSpace(lines[i])
			}
		}
	}
	return "pulled"
}

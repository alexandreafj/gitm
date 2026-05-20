package cli

import (
	"fmt"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func runBranchCreateWithUI(ui ui, args []string, selectAll bool, fromBranch string, repoAliases []string) error {
	branchName := args[0]

	allRepos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	var chosen []*db.Repository
	switch {
	case len(repoAliases) > 0:
		// --repo provided: use resolved repos directly, no prompt.
		chosen = allRepos
	case selectAll:
		chosen = allRepos
	default:
		chosen, err = ui.MultiSelect(
			allRepos,
			fmt.Sprintf("Select repositories for new branch: %s", branchName),
			false,
			nil,
		)
		if err != nil {
			return err
		}
	}

	fmt.Printf("\nCreating branch %q in %d repository(ies)…\n\n", branchName, len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		base := repo.DefaultBranch
		if fromBranch != "" {
			base = fromBranch
		}

		dirty, err := git.IsDirtyTrackedOnly(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("git status: %w", err)
		}
		if dirty {
			return "", "uncommitted changes — stash or commit first", nil
		}

		if err := git.Checkout(repo.Path, base); err != nil {
			return "", "", fmt.Errorf("checkout %s: %w", base, err)
		}
		if _, err := git.Pull(repo.Path); err != nil {
			fmt.Printf("  warning: pull failed on %s: %v\n", repo.Alias, err)
		}

		if git.BranchExists(repo.Path, branchName) {
			if err := git.Checkout(repo.Path, branchName); err != nil {
				return "", "", fmt.Errorf("checkout existing branch: %w", err)
			}
			return fmt.Sprintf("branch %s already exists — checked out", branchName), "", nil
		}

		if err := git.CreateBranch(repo.Path, branchName); err != nil {
			return "", "", fmt.Errorf("create branch: %w", err)
		}

		return fmt.Sprintf("created %s from %s", branchName, base), "", nil
	})

	return nil
}

func runBranchRenameWithUI(ui ui, oldName, newName string, selectAll, noRemote bool, repoAliases []string) error {
	allRepos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	var reposWithBranch []*db.Repository
	for _, r := range allRepos {
		if git.BranchExists(r.Path, oldName) {
			reposWithBranch = append(reposWithBranch, r)
		}
	}

	if len(reposWithBranch) == 0 {
		return fmt.Errorf("no registered repositories have a branch named %q", oldName)
	}

	var chosen []*db.Repository
	switch {
	case len(repoAliases) > 0:
		// --repo provided: use repos-with-branch subset directly, no prompt.
		chosen = reposWithBranch
	case selectAll:
		chosen = reposWithBranch
	default:
		chosen, err = ui.MultiSelect(
			reposWithBranch,
			fmt.Sprintf("Select repositories to rename: %s → %s", oldName, newName),
			false,
			nil,
		)
		if err != nil {
			return err
		}
	}

	fmt.Printf("\nRenaming %q → %q in %d repository(ies)…\n\n", oldName, newName, len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		if err := git.RenameBranch(repo.Path, oldName, newName); err != nil {
			return "", "", fmt.Errorf("local rename: %w", err)
		}

		if noRemote {
			return fmt.Sprintf("renamed %s → %s (local only)", oldName, newName), "", nil
		}

		if git.RemoteBranchExists(repo.Path, oldName) {
			if err := git.DeleteRemoteBranch(repo.Path, oldName); err != nil {
				return "", "", fmt.Errorf("delete remote branch %s: %w", oldName, err)
			}
		}

		if err := git.PushBranch(repo.Path, newName); err != nil {
			return "", "", fmt.Errorf("push %s: %w", newName, err)
		}

		return fmt.Sprintf("renamed %s → %s (local + remote)", oldName, newName), "", nil
	})

	return nil
}

func runBranchDeleteWithUI(ui ui, branchName string, selectAll, force, noRemote bool, repoAliases []string) error {
	allRepos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	var reposWithBranch []*db.Repository
	for _, r := range allRepos {
		has := git.BranchExists(r.Path, branchName)
		if !has && !noRemote {
			has = git.RemoteBranchExists(r.Path, branchName)
		}
		if has {
			reposWithBranch = append(reposWithBranch, r)
		}
	}

	if len(reposWithBranch) == 0 {
		return fmt.Errorf("no registered repositories have a branch named %q", branchName)
	}

	var chosen []*db.Repository
	switch {
	case len(repoAliases) > 0 || selectAll:
		// --repo / --all: non-interactive, so confirm explicitly before deleting.
		chosen = reposWithBranch
		fmt.Printf("Branch %q will be deleted in %d repository(ies):\n", branchName, len(chosen))
		for _, r := range chosen {
			fmt.Printf("  - %s\n", r.Alias)
		}
		confirmed, confErr := ui.Confirm(fmt.Sprintf("Delete branch %q? [y/N]", branchName))
		if confErr != nil {
			return confErr
		}
		if !confirmed {
			fmt.Println("Aborted — no branches deleted.")
			return nil
		}
	default:
		chosen, err = ui.MultiSelect(
			reposWithBranch,
			fmt.Sprintf("Select repositories to delete branch: %s", branchName),
			false,
			nil,
		)
		if err != nil {
			return err
		}
	}

	fmt.Printf("\nDeleting %q in %d repository(ies)…\n\n", branchName, len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		if branchName == repo.DefaultBranch {
			return "", fmt.Sprintf("refusing to delete the default branch %q", branchName), nil
		}

		current, err := git.CurrentBranch(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("current branch: %w", err)
		}
		if current == branchName {
			return "", "branch is currently checked out — switch away first", nil
		}

		var deleted []string
		if git.BranchExists(repo.Path, branchName) {
			if err := git.DeleteLocalBranch(repo.Path, branchName, force); err != nil {
				if !force {
					return "", "", fmt.Errorf("local delete failed — branch may have unmerged commits, re-run with --force: %w", err)
				}
				return "", "", fmt.Errorf("local delete: %w", err)
			}
			deleted = append(deleted, "local")
		}

		if !noRemote && git.RemoteBranchExists(repo.Path, branchName) {
			if err := git.DeleteRemoteBranch(repo.Path, branchName); err != nil {
				return "", "", fmt.Errorf("remote delete: %w", err)
			}
			deleted = append(deleted, "remote")
		}

		if len(deleted) == 0 {
			return "", fmt.Sprintf("branch %q not found", branchName), nil
		}

		return fmt.Sprintf("deleted %s (%s)", branchName, strings.Join(deleted, " + ")), "", nil
	})

	return nil
}

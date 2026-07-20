package cli

import (
	"fmt"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func runBranchCreateWithUI(ui ui, args []string, selectAll bool, fromBranch string, repoAliases []string, noRemote bool) error {
	return runBranchCreateWithUIAndGroup(ui, args, selectAll, fromBranch, repoAliases, "", noRemote)
}

func runBranchCreateWithUIAndGroup(ui ui, args []string, selectAll bool, fromBranch string, repoAliases []string, groupName string, noRemote bool) error {
	branchName := args[0]

	allRepos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
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

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
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
		baseNote := ""
		hasUpstream, err := git.HasUpstream(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("check upstream for %s: %w", base, err)
		}
		if hasUpstream {
			if _, err := git.Pull(repo.Path); err != nil {
				return "", "", fmt.Errorf("refresh base %s: %w", base, err)
			}
		} else {
			baseNote = " (base has no upstream; used local state)"
		}

		if git.BranchExists(repo.Path, branchName) {
			if err := git.Checkout(repo.Path, branchName); err != nil {
				return "", "", fmt.Errorf("checkout existing branch: %w", err)
			}
			note, err := ensureBranchUpstream(repo.Path, branchName, noRemote)
			if err != nil {
				return "", "", err
			}
			return fmt.Sprintf("branch %s already exists — checked out%s%s", branchName, note, baseNote), "", nil
		}

		if err := git.CreateBranch(repo.Path, branchName); err != nil {
			return "", "", fmt.Errorf("create branch: %w", err)
		}

		note, err := ensureBranchUpstream(repo.Path, branchName, noRemote)
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf("created %s from %s%s%s", branchName, base, note, baseNote), "", nil
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to create branch", runner.ErrorCount(results))
	}
	return nil
}

// ensureBranchUpstream pushes branch to origin with upstream tracking so that
// later pulls (gitm update, gitm checkout) don't fail with "no tracking
// information". Without it a branch created by gitm only gains an upstream
// once gitm commit pushes it. Branches that already track a remote are left
// untouched, and repositories without an origin remote skip the push.
// The returned note is appended to the runner's per-repo success message.
func ensureBranchUpstream(path, branch string, noRemote bool) (string, error) {
	if noRemote {
		return " (local only)", nil
	}
	hasOrigin, err := git.RemoteConfigured(path, "origin")
	if err != nil {
		return "", fmt.Errorf("check origin remote: %w", err)
	}
	if !hasOrigin {
		return " (no origin remote — not pushed)", nil
	}
	if hasUpstream, upErr := git.HasUpstream(path); upErr == nil && hasUpstream {
		return "", nil
	}
	if err := git.PushBranch(path, branch); err != nil {
		return "", fmt.Errorf("push %s to origin: %w", branch, err)
	}
	return fmt.Sprintf(" — tracking origin/%s", branch), nil
}

func runBranchRenameWithUI(ui ui, oldName, newName string, selectAll, noRemote bool, repoAliases []string) error {
	return runBranchRenameWithUIAndGroup(ui, oldName, newName, selectAll, noRemote, repoAliases, "")
}

func runBranchRenameWithUIAndGroup(ui ui, oldName, newName string, selectAll, noRemote bool, repoAliases []string, groupName string) error {
	allRepos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
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

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		if err := git.RenameBranch(repo.Path, oldName, newName); err != nil {
			return "", "", fmt.Errorf("local rename: %w", err)
		}

		if noRemote {
			return fmt.Sprintf("renamed %s → %s (local only)", oldName, newName), "", nil
		}

		oldRemoteExists := git.RemoteBranchExists(repo.Path, oldName)
		if err := git.PushBranch(repo.Path, newName); err != nil {
			return "", "", fmt.Errorf("push %s: %w", newName, err)
		}
		if oldRemoteExists {
			if err := git.DeleteRemoteBranch(repo.Path, oldName); err != nil {
				return "", "", fmt.Errorf("new branch %s is published, but delete old remote branch %s: %w", newName, oldName, err)
			}
		}

		return fmt.Sprintf("renamed %s → %s (local + remote)", oldName, newName), "", nil
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to rename branch", runner.ErrorCount(results))
	}
	return nil
}

func runBranchDeleteWithUI(ui ui, branchName string, selectAll, force, noRemote bool, repoAliases []string) error {
	return runBranchDeleteWithUIAndGroup(ui, branchName, selectAll, force, noRemote, repoAliases, "")
}

func runBranchDeleteWithUIAndGroup(ui ui, branchName string, selectAll, force, noRemote bool, repoAliases []string, groupName string) error {
	return runBranchDeleteWithUIAndGroupDryRun(ui, branchName, selectAll, force, noRemote, repoAliases, groupName, false)
}

func runBranchDeleteWithUIDryRun(ui ui, branchName string, selectAll, force, noRemote bool, repoAliases []string, dryRun bool) error {
	return runBranchDeleteWithUIAndGroupDryRun(ui, branchName, selectAll, force, noRemote, repoAliases, "", dryRun)
}

func runBranchDeleteWithUIAndGroupDryRun(ui ui, branchName string, selectAll, force, noRemote bool, repoAliases []string, groupName string, dryRun bool) error {
	allRepos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
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
		chosen = reposWithBranch
		if dryRun {
			break
		}
		// --repo / --all: non-interactive, so confirm explicitly before deleting.
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

	if dryRun {
		printDryRunPreview(
			fmt.Sprintf("Branch %q delete preview for %d repository(ies)", branchName, len(chosen)),
			branchDeleteDryRunItems(chosen, branchName, force, noRemote),
		)
		return nil
	}

	fmt.Printf("\nDeleting %q in %d repository(ies)…\n\n", branchName, len(chosen))

	results := runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		if branchName == repo.DefaultBranch {
			return "", fmt.Sprintf("refusing to delete the default branch %q", branchName), nil
		}

		current, err := git.CurrentBranch(repo.Path)
		if err != nil {
			return "", "", fmt.Errorf("current branch: %w", err)
		}
		switchedTo := ""
		if current == branchName {
			if err := git.Checkout(repo.Path, repo.DefaultBranch); err != nil {
				return "", "", fmt.Errorf("checkout default branch %s: %w", repo.DefaultBranch, err)
			}
			switchedTo = repo.DefaultBranch
		}

		var deleted []string
		if git.BranchExists(repo.Path, branchName) {
			if err := git.DeleteLocalBranch(repo.Path, branchName, force); err != nil {
				if !force {
					return "", "branch has unmerged commits — re-run with --force to delete anyway", nil
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

		result := fmt.Sprintf("deleted %s (%s)", branchName, strings.Join(deleted, " + "))
		if switchedTo != "" {
			result = fmt.Sprintf("switched to %s — %s", switchedTo, result)
		}
		return result, "", nil
	})

	if runner.HasErrors(results) {
		return fmt.Errorf("%d repository(ies) failed to delete branch", runner.ErrorCount(results))
	}
	return nil
}

func branchDeleteDryRunItems(repos []*db.Repository, branchName string, force, noRemote bool) []dryRunItem {
	items := make([]dryRunItem, 0, len(repos))
	for _, repo := range repos {
		item := dryRunItem{repo: repo}

		if branchName == repo.DefaultBranch {
			item.skipReason = fmt.Sprintf("refusing to delete the default branch %q", branchName)
			items = append(items, item)
			continue
		}

		current, err := git.CurrentBranch(repo.Path)
		if err != nil {
			item.skipReason = fmt.Sprintf("current branch: %v", err)
			items = append(items, item)
			continue
		}
		if current == branchName {
			item.actions = append(item.actions, fmt.Sprintf("git checkout %s", repo.DefaultBranch))
		}

		localExists := git.BranchExists(repo.Path, branchName)
		remoteExists := false
		if !noRemote {
			remoteExists = git.RemoteBranchExists(repo.Path, branchName)
		}
		if !localExists && !remoteExists {
			item.skipReason = fmt.Sprintf("branch %q not found", branchName)
			items = append(items, item)
			continue
		}

		if localExists && !force {
			merged, mergeErr := git.BranchMerged(repo.Path, branchName)
			if mergeErr != nil {
				item.warning = fmt.Sprintf("could not confirm whether %q is merged: %v", branchName, mergeErr)
			} else if !merged {
				item.skipReason = "branch has unmerged commits — re-run with --force to delete anyway"
				items = append(items, item)
				continue
			}
		}

		if localExists {
			flag := "-d"
			if force {
				flag = "-D"
			}
			item.actions = append(item.actions, fmt.Sprintf("git branch %s %s", flag, branchName))
		}
		if remoteExists {
			item.actions = append(item.actions, fmt.Sprintf("git push origin --delete %s", branchName))
		}

		items = append(items, item)
	}
	return items
}

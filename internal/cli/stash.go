package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

func stashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Stash and apply changes across selected repositories",
		Long: `Manage git stashes across multiple repositories.

Subcommands:
  gitm stash        — select dirty repos and stash their changes
  gitm stash apply  — select repos with stashes and apply the latest
  gitm stash pop    — select repos with stashes, apply and drop the latest
  gitm stash list   — show all repos that have stash entries`,
		Args: cobra.NoArgs,
		RunE: runStashPush,
	}

	cmd.AddCommand(stashApplyCmd())
	cmd.AddCommand(stashPopCmd())
	cmd.AddCommand(stashListCmd())

	return cmd
}

func runStashPush(cmd *cobra.Command, args []string) error {
	return runStashPushWithUI(liveUI{})
}

func runStashPushWithUI(ui ui) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	// Filter to dirty repos.
	fmt.Println("Scanning repositories for uncommitted changes…")
	var dirty []*db.Repository
	for _, repo := range repos {
		d, dirtyErr := git.IsDirty(repo.Path)
		if dirtyErr != nil {
			color.Yellow("  ⚠  %s: cannot check status (%v) — skipping", repo.Alias, dirtyErr)
			continue
		}
		if d {
			dirty = append(dirty, repo)
		}
	}

	if len(dirty) == 0 {
		fmt.Println("Nothing to stash — all repositories are clean.")
		return nil
	}

	chosen, err := ui.MultiSelect(dirty, "Select repositories to stash", false, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("\nStashing changes in %d repository(ies)…\n\n", len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		branch, err := git.CurrentBranch(repo.Path)
		if err != nil {
			branch = "unknown"
		}
		msg := fmt.Sprintf("gitm stash on %s", branch)
		if err := git.StashPush(repo.Path, msg); err != nil {
			return "", "", fmt.Errorf("git stash: %w", err)
		}
		return fmt.Sprintf("stashed (%s)", msg), "", nil
	})

	return nil
}

func stashApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply the latest stash in selected repositories (keeps stash)",
		Args:  cobra.NoArgs,
		RunE:  runStashApply,
	}
}

func runStashApply(cmd *cobra.Command, args []string) error {
	return runStashApplyOrPopWithUI(liveUI{}, false)
}

func stashPopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pop",
		Short: "Apply and drop the latest stash in selected repositories",
		Args:  cobra.NoArgs,
		RunE:  runStashPop,
	}
}

func runStashPop(cmd *cobra.Command, args []string) error {
	return runStashApplyOrPopWithUI(liveUI{}, true)
}

func runStashApplyOrPopWithUI(ui ui, pop bool) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	// Filter to repos that have stash entries.
	fmt.Println("Scanning repositories for stash entries…")
	var withStash []*db.Repository
	for _, repo := range repos {
		has, stashErr := git.HasStash(repo.Path)
		if stashErr != nil {
			color.Yellow("  ⚠  %s: cannot check stash (%v) — skipping", repo.Alias, stashErr)
			continue
		}
		if has {
			withStash = append(withStash, repo)
		}
	}

	if len(withStash) == 0 {
		fmt.Println("No repositories have stash entries.")
		return nil
	}

	verb := "apply"
	if pop {
		verb = "pop"
	}
	title := fmt.Sprintf("Select repositories to stash %s", verb)

	chosen, err := ui.MultiSelect(withStash, title, false, nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("\nRunning stash %s in %d repository(ies)…\n\n", verb, len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		var opErr error
		if pop {
			opErr = git.StashPop(repo.Path)
		} else {
			opErr = git.StashApply(repo.Path)
		}
		if opErr != nil {
			return "", "", fmt.Errorf("git stash %s: %w", verb, opErr)
		}
		return fmt.Sprintf("stash %s applied", verb), "", nil
	})

	return nil
}

func stashListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show repositories that have stash entries",
		Args:  cobra.NoArgs,
		RunE:  runStashList,
	}
}

func runStashList(cmd *cobra.Command, args []string) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	type stashEntry struct {
		repo    *db.Repository
		entries []string
	}

	var found []stashEntry
	for _, repo := range repos {
		entries, err := git.StashList(repo.Path)
		if err != nil || len(entries) == 0 {
			continue
		}
		found = append(found, stashEntry{repo: repo, entries: entries})
	}

	if len(found) == 0 {
		fmt.Println("No repositories have stash entries.")
		return nil
	}

	// Calculate column widths.
	aliasW := len("REPO")
	for _, e := range found {
		if len(e.repo.Alias) > aliasW {
			aliasW = len(e.repo.Alias)
		}
	}

	header := color.New(color.Bold)
	header.Printf("%-*s  %-7s  %s\n", aliasW, "REPO", "STASHES", "TOP STASH")
	fmt.Println(strings.Repeat("─", aliasW+2+7+2+60))

	for _, e := range found {
		// Trim the stash ref prefix from the top entry for readability.
		// "stash@{0}: On branch: gitm stash on feature/X" → "gitm stash on feature/X"
		top := e.entries[0]
		if idx := strings.Index(top, ": "); idx >= 0 {
			top = top[idx+2:]
		}
		if len(top) > 60 {
			top = top[:57] + "…"
		}
		fmt.Printf("%-*s  %-7d  %s\n", aliasW, e.repo.Alias, len(e.entries), top)
	}

	fmt.Printf("\n%d repository(ies) with stash entries.\n", len(found))
	return nil
}

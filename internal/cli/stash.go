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
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Stash and apply changes across selected repositories",
		Long: `Manage git stashes across multiple repositories.

Subcommands:
  gitm stash        — select dirty repos and stash their changes
  gitm stash apply  — select repos with stashes and apply the latest
  gitm stash pop    — select repos with stashes, apply and drop the latest
  gitm stash list   — show all repos that have stash entries

Use --repo / -r to target specific repositories by alias, bypassing the
interactive multi-select UI entirely. Non-dirty repos are silently skipped.`,
		Example: `  gitm stash
  gitm stash -r api-gateway
  gitm stash -r api-gateway,auth-service
  gitm stash apply -r api-gateway
  gitm stash pop --repo=api-gateway,auth-service
  gitm stash list -r api-gateway`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStashPush(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")

	cmd.AddCommand(stashApplyCmd())
	cmd.AddCommand(stashPopCmd())
	cmd.AddCommand(stashListCmd())

	return cmd
}

func runStashPush(repoAliases []string) error {
	return runStashPushWithUI(liveUI{}, repoAliases)
}

func runStashPushWithUI(ui ui, repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
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

	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		chosen = dirty
	} else {
		chosen, err = ui.MultiSelect(dirty, "Select repositories to stash", false, nil)
		if err != nil {
			fmt.Println(err)
			return nil
		}
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
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the latest stash in selected repositories (keeps stash)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStashApplyOrPop(false, repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")
	return cmd
}

func stashPopCmd() *cobra.Command {
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "pop",
		Short: "Apply and drop the latest stash in selected repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStashApplyOrPop(true, repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")
	return cmd
}

func runStashApplyOrPop(pop bool, repoAliases []string) error {
	return runStashApplyOrPopWithUI(liveUI{}, pop, repoAliases)
}

func runStashApplyOrPopWithUI(ui ui, pop bool, repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
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

	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		chosen = withStash
	} else {
		chosen, err = ui.MultiSelect(withStash, title, false, nil)
		if err != nil {
			fmt.Println(err)
			return nil
		}
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
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show repositories that have stash entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStashListFn(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	return cmd
}

func runStashListFn(repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
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

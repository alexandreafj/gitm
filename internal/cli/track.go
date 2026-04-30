package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func trackCmd() *cobra.Command {
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Start tracking untracked files across repositories",
		Long: `Interactively select untracked files to start tracking (git add) across
multiple repositories. Only repositories with untracked files are shown.

Use --repo to target specific repositories by alias, bypassing the interactive
multi-select UI.`,
		Example: `  gitm track
  gitm track --repo api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrack(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")

	return cmd
}

func runTrack(repoAliases []string) error {
	return runTrackWithUI(liveUI{}, repoAliases)
}

type trackResult struct {
	alias   string
	skipped bool
	err     error
}

func runTrackWithUI(ui ui, repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories registered — run `gitm repo add <path>` first")
	}

	var withUntracked []*db.Repository
	for _, repo := range repos {
		files, filesErr := git.UntrackedFiles(repo.Path)
		if filesErr != nil {
			color.Yellow("  ⚠  %s: cannot check status (%v) — skipping", repo.Alias, filesErr)
			continue
		}
		if len(files) > 0 {
			withUntracked = append(withUntracked, repo)
		}
	}

	if len(withUntracked) == 0 {
		color.Green("No untracked files found in any repository.")
		return nil
	}

	fmt.Printf("%s repositories with untracked files:\n\n", color.YellowString("%d", len(withUntracked)))
	for _, repo := range withUntracked {
		files, filesErr := git.UntrackedFiles(repo.Path)
		if filesErr != nil {
			continue
		}
		fmt.Printf("  %s  %s\n",
			color.CyanString("%-22s", repo.Alias),
			color.New(color.FgWhite).Sprintf("%d untracked file(s)", len(files)),
		)
	}
	fmt.Println()

	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		chosen = withUntracked
	} else {
		chosen, err = ui.MultiSelect(
			withUntracked,
			"Select repositories to track files in",
			false,
			nil,
		)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}

	results := make([]trackResult, 0, len(chosen))

	for _, repo := range chosen {
		fmt.Printf("\n%s\n", color.CyanString("━━━ %s ━━━", repo.Alias))

		files, filesErr := git.UntrackedFiles(repo.Path)
		if filesErr != nil {
			color.Red("  ✗ Cannot list untracked files: %v", filesErr)
			results = append(results, trackResult{alias: repo.Alias, err: filesErr})
			continue
		}
		if len(files) == 0 {
			color.Yellow("  ⚠  No untracked files found — skipping")
			results = append(results, trackResult{alias: repo.Alias, skipped: true})
			continue
		}

		selectedFiles, selectErr := ui.FileSelect(
			files,
			fmt.Sprintf("Select files to track for %s", repo.Alias),
		)
		if selectErr != nil {
			if selectErr.Error() == "canceled" || selectErr.Error() == "no files selected" {
				color.Yellow("  ⚠  Skipped (%s)", selectErr)
				results = append(results, trackResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ File selection error: %v", selectErr)
			results = append(results, trackResult{alias: repo.Alias, err: selectErr})
			continue
		}

		if stageErr := git.StageFiles(repo.Path, selectedFiles); stageErr != nil {
			color.Red("  ✗ git add failed: %v", stageErr)
			results = append(results, trackResult{alias: repo.Alias, err: stageErr})
			continue
		}

		color.Green("  ✓ Tracked %d file(s)", len(selectedFiles))
		results = append(results, trackResult{alias: repo.Alias})
	}

	fmt.Println()
	fmt.Println(color.New(color.Bold).Sprint("Summary"))
	fmt.Println(color.New(color.FgHiBlack).Sprint("───────────────────────"))

	succeeded, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch {
		case r.skipped:
			skipped++
			fmt.Printf("  %s  %s\n", color.YellowString("~"), r.alias)
		case r.err != nil:
			failed++
			fmt.Printf("  %s  %s: %v\n", color.RedString("✗"), r.alias, r.err)
		default:
			succeeded++
			fmt.Printf("  %s  %s\n", color.GreenString("✓"), r.alias)
		}
	}

	fmt.Println()
	fmt.Printf("%s  %s  %s\n",
		color.GreenString("%d tracked", succeeded),
		color.YellowString("%d skipped", skipped),
		color.RedString("%d failed", failed),
	)

	return nil
}

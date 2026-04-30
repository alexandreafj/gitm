package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
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
			true,
			nil,
		)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}

	fmt.Printf("\nTracking files in %d repository(ies)…\n\n", len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		files, filesErr := git.UntrackedFiles(repo.Path)
		if filesErr != nil {
			return "", "", fmt.Errorf("list untracked files: %w", filesErr)
		}
		if len(files) == 0 {
			return "", "no untracked files", nil
		}

		selectedFiles, selectErr := ui.FileSelect(
			files,
			fmt.Sprintf("Select files to track for %s", repo.Alias),
		)
		if selectErr != nil {
			return "", selectErr.Error(), nil
		}

		if stageErr := git.StageFiles(repo.Path, selectedFiles); stageErr != nil {
			return "", "", fmt.Errorf("git add failed: %w", stageErr)
		}

		return fmt.Sprintf("tracked %d file(s)", len(selectedFiles)), "", nil
	})

	return nil
}

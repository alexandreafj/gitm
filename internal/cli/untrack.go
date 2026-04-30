package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
)

func untrackCmd() *cobra.Command {
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "untrack",
		Short: "Stop tracking files across repositories (keeps files on disk)",
		Long: `Interactively select tracked files to stop tracking (git rm --cached) across
multiple repositories. Files are removed from the git index but remain on disk.

This is useful for accidentally committed files like .env, logs, or build artifacts.

Use --repo to target specific repositories by alias, bypassing the interactive
multi-select UI.`,
		Example: `  gitm untrack
  gitm untrack --repo api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUntrack(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")

	return cmd
}

func runUntrack(repoAliases []string) error {
	return runUntrackWithUI(liveUI{}, repoAliases)
}

func runUntrackWithUI(ui ui, repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories registered — run `gitm repo add <path>` first")
	}

	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		chosen = repos
	} else {
		chosen, err = ui.MultiSelect(
			repos,
			"Select repositories to untrack files from",
			false,
			nil,
		)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}

	fmt.Printf("\nUntracking files in %d repository(ies)…\n\n", len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		files, filesErr := git.TrackedFiles(repo.Path)
		if filesErr != nil {
			return "", "", fmt.Errorf("list tracked files: %w", filesErr)
		}
		if len(files) == 0 {
			return "", "no tracked files", nil
		}

		selectedFiles, selectErr := ui.FileSelect(
			files,
			fmt.Sprintf("Select files to untrack for %s (files stay on disk)", repo.Alias),
		)
		if selectErr != nil {
			return "", selectErr.Error(), nil
		}

		if untrackErr := git.UntrackFiles(repo.Path, selectedFiles); untrackErr != nil {
			return "", "", fmt.Errorf("git rm --cached failed: %w", untrackErr)
		}

		return fmt.Sprintf("untracked %d file(s)", len(selectedFiles)), "", nil
	})

	return nil
}

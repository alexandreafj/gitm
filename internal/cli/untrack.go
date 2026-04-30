package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/runner"
)

func untrackCmd() *cobra.Command {
	var (
		repoAliases []string
		pathFilter  string
	)

	cmd := &cobra.Command{
		Use:   "untrack",
		Short: "Stop tracking files across repositories (keeps files on disk)",
		Long: `Interactively select tracked files to stop tracking (git rm --cached) across
multiple repositories. Files are removed from the git index but remain on disk.

This is useful for accidentally committed files like .env, logs, or build artifacts.

Use --path to filter which tracked files are shown (supports glob patterns and
path prefixes). Without --path, all tracked files are listed.

Use --repo to target specific repositories by alias, bypassing the interactive
multi-select UI.`,
		Example: `  gitm untrack
  gitm untrack --path "*.env"
  gitm untrack --path "public/"
  gitm untrack --path "debug.log"
  gitm untrack --repo api-gateway --path "*.log"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUntrack(repoAliases, pathFilter)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	cmd.Flags().StringVarP(&pathFilter, "path", "p", "", "Filter files by glob pattern or path prefix (e.g. \"*.env\", \"public/\")")

	return cmd
}

func runUntrack(repoAliases []string, pathFilter string) error {
	return runUntrackWithUI(liveUI{}, repoAliases, pathFilter)
}

func filterTrackedFiles(files []string, pattern string) []string {
	if pattern == "" {
		return files
	}

	var matched []string
	for _, f := range files {
		var path string
		if len(f) > 3 {
			path = strings.TrimSpace(f[3:])
		} else {
			path = strings.TrimSpace(f)
		}

		if strings.HasPrefix(path, pattern) {
			matched = append(matched, f)
			continue
		}

		base := filepath.Base(path)
		if ok, err := filepath.Match(pattern, base); err == nil && ok {
			matched = append(matched, f)
			continue
		}

		if ok, err := filepath.Match(pattern, path); err == nil && ok {
			matched = append(matched, f)
		}
	}
	return matched
}

func runUntrackWithUI(ui ui, repoAliases []string, pathFilter string) error {
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

		files = filterTrackedFiles(files, pathFilter)
		if len(files) == 0 {
			return "", fmt.Sprintf("no files matching %q", pathFilter), nil
		}

		title := fmt.Sprintf("Select files to untrack for %s (files stay on disk)", repo.Alias)
		if pathFilter != "" {
			title = fmt.Sprintf("Select files to untrack for %s [filter: %s] (files stay on disk)", repo.Alias, pathFilter)
		}

		selectedFiles, selectErr := ui.FileSelect(files, title)
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

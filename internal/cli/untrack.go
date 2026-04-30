package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
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

type untrackResult struct {
	alias   string
	skipped bool
	err     error
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

	results := make([]untrackResult, 0, len(chosen))

	for _, repo := range chosen {
		fmt.Printf("\n%s\n", color.CyanString("━━━ %s ━━━", repo.Alias))

		files, filesErr := git.TrackedFiles(repo.Path)
		if filesErr != nil {
			color.Red("  ✗ Cannot list tracked files: %v", filesErr)
			results = append(results, untrackResult{alias: repo.Alias, err: filesErr})
			continue
		}
		if len(files) == 0 {
			color.Yellow("  ⚠  No tracked files found — skipping")
			results = append(results, untrackResult{alias: repo.Alias, skipped: true})
			continue
		}

		files = filterTrackedFiles(files, pathFilter)
		if len(files) == 0 {
			color.Yellow("  ⚠  No files matching %q — skipping", pathFilter)
			results = append(results, untrackResult{alias: repo.Alias, skipped: true})
			continue
		}

		title := fmt.Sprintf("Select files to untrack for %s (files stay on disk)", repo.Alias)
		if pathFilter != "" {
			title = fmt.Sprintf("Select files to untrack for %s [filter: %s] (files stay on disk)", repo.Alias, pathFilter)
		}

		selectedFiles, selectErr := ui.FileSelect(files, title)
		if selectErr != nil {
			if selectErr.Error() == "canceled" || selectErr.Error() == "no files selected" {
				color.Yellow("  ⚠  Skipped (%s)", selectErr)
				results = append(results, untrackResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ File selection error: %v", selectErr)
			results = append(results, untrackResult{alias: repo.Alias, err: selectErr})
			continue
		}

		if untrackErr := git.UntrackFiles(repo.Path, selectedFiles); untrackErr != nil {
			color.Red("  ✗ git rm --cached failed: %v", untrackErr)
			results = append(results, untrackResult{alias: repo.Alias, err: untrackErr})
			continue
		}

		color.Green("  ✓ Untracked %d file(s)", len(selectedFiles))
		results = append(results, untrackResult{alias: repo.Alias})
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
		color.GreenString("%d untracked", succeeded),
		color.YellowString("%d skipped", skipped),
		color.RedString("%d failed", failed),
	)

	return nil
}

package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func discardCmd() *cobra.Command {
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "discard",
		Short: "Interactively select files to discard in each repository",
		Long: `Interactively select which repositories and files to discard changes in.
Only repositories with uncommitted changes are shown — if none have changes,
the command exits with a message.

For each selected repository you choose exactly which files to discard.
No files are pre-selected — you must explicitly pick every file you want gone.

Depending on the file status, the appropriate git command is used:
  Modified tracked files  → git checkout -- <file>
  Staged new files        → git reset HEAD -- <file> + git clean -f -- <file>
  Untracked files         → git clean -f -- <file>

Use --repo / -r to target specific repositories by alias, bypassing the
interactive multi-select UI entirely. Non-dirty repos are silently skipped.

WARNING: This operation is irreversible. Discarded changes cannot be recovered.`,
		Example: `  gitm discard
  gitm discard --repo my-api
  gitm discard -r api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiscard(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")

	return cmd
}

// discardResult holds the outcome for a single repo.
type discardResult struct {
	alias   string
	files   int
	skipped bool
	err     error
}

func runDiscard(repoAliases []string) error {
	return runDiscardWithUI(liveUI{}, repoAliases)
}

func runDiscardWithUI(ui ui, repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	// Filter to only repos that actually have uncommitted changes.
	var dirtyRepos []*db.Repository
	for _, repo := range repos {
		dirty, dirtyErr := git.IsDirty(repo.Path)
		if dirtyErr != nil {
			color.Yellow("  ⚠ %s: could not check status: %v", repo.Alias, dirtyErr)
			continue
		}
		if dirty {
			dirtyRepos = append(dirtyRepos, repo)
		}
	}

	if len(dirtyRepos) == 0 {
		color.Green("Nothing to discard — all repositories are clean.")
		return nil
	}

	// Show a summary of what's dirty before prompting.
	fmt.Printf("%s repositories with uncommitted changes:\n\n", color.YellowString("%d", len(dirtyRepos)))
	for _, repo := range dirtyRepos {
		files, filesErr := git.DirtyFiles(repo.Path)
		if filesErr != nil {
			color.Yellow("  ⚠ %s: could not list dirty files: %v", repo.Alias, filesErr)
			continue
		}
		fmt.Printf("  %s  %s\n",
			color.CyanString("%-22s", repo.Alias),
			color.New(color.FgWhite).Sprintf("%d file(s) modified", len(files)),
		)
	}
	fmt.Println()

	// Step 1: Select repos — skip MultiSelect when --repo is provided.
	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		chosen = dirtyRepos
	} else {
		chosen, err = ui.MultiSelect(
			dirtyRepos,
			"WARNING: Select repositories to discard changes in (irreversible)",
			false,
			nil,
		)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}

	// Step 2: Sequential per-repo file selection + discard.
	results := make([]discardResult, 0, len(chosen))

	for _, repo := range chosen {
		fmt.Printf("\n%s\n", color.CyanString("━━━ %s ━━━", repo.Alias))

		// Get dirty files with status.
		porcelainLines, filesErr := git.DirtyFilesWithStatus(repo.Path)
		if filesErr != nil {
			color.Red("  ✗ Cannot list dirty files: %v", filesErr)
			results = append(results, discardResult{alias: repo.Alias, err: filesErr})
			continue
		}
		if len(porcelainLines) == 0 {
			color.Yellow("  ⚠  No dirty files found (may have been cleaned externally) — skipping")
			results = append(results, discardResult{alias: repo.Alias, skipped: true})
			continue
		}

		// File picker — nothing pre-selected.
		selectedFiles, selectErr := ui.FileSelect(
			porcelainLines,
			fmt.Sprintf("Select files to discard in %s (irreversible)", repo.Alias),
		)
		if selectErr != nil {
			if selectErr.Error() == "canceled" || selectErr.Error() == "no files selected" {
				color.Yellow("  ⚠  Skipped (no files selected)")
				results = append(results, discardResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ File selection error: %v", selectErr)
			results = append(results, discardResult{alias: repo.Alias, err: selectErr})
			continue
		}

		// Discard selected files.
		if discardErr := git.DiscardFiles(repo.Path, selectedFiles); discardErr != nil {
			color.Red("  ✗ Discard failed: %v", discardErr)
			results = append(results, discardResult{alias: repo.Alias, err: discardErr})
			continue
		}

		// Report what was discarded.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("  ✓ Discarded %d file(s):\n", len(selectedFiles)))
		for _, f := range selectedFiles {
			// Strip porcelain prefix for display.
			name := f
			if len(f) > 3 {
				name = strings.TrimSpace(f[3:])
			}
			sb.WriteString(fmt.Sprintf("       %s\n", color.New(color.FgWhite).Sprint(name)))
		}
		fmt.Print(sb.String())

		results = append(results, discardResult{alias: repo.Alias, files: len(selectedFiles)})
	}

	// Step 3: Summary.
	fmt.Println()
	fmt.Println(color.New(color.Bold).Sprint("Summary"))
	fmt.Println(color.New(color.FgHiBlack).Sprint("───────────────────────"))

	succeeded := 0
	failed := 0
	skipped := 0
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
			fmt.Printf("  %s  %s (%d file(s) discarded)\n", color.GreenString("✓"), r.alias, r.files)
		}
	}

	fmt.Println()
	fmt.Printf("%s  %s  %s\n",
		color.GreenString("%d discarded", succeeded),
		color.YellowString("%d skipped", skipped),
		color.RedString("%d failed", failed),
	)

	return nil
}

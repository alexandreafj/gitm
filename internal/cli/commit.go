package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
	"github.com/alexandreferreira/gitm/internal/tui"
)

func commitCmd() *cobra.Command {
	var noPush bool

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Interactively stage files and commit across dirty repositories",
		Long: `gitm commit scans all registered repositories for uncommitted changes,
lets you pick which repos to commit, then walks through each one sequentially:
  1. Select files to stage
  2. Enter a commit message
  3. Stage selected files, commit, and push (use --no-push to skip push)

Repositories on their default branch are shown but cannot be selected (protected).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(noPush)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip git push after committing")

	return cmd
}

// repoCommitResult holds the outcome for a single repo.
type repoCommitResult struct {
	alias   string
	skipped bool
	err     error
	pushed  bool
}

func runCommit(noPush bool) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories registered — run `gitm repo add <path>` first")
	}

	// Step 1: Scan repos for dirty state and protected-branch status.
	fmt.Println("Scanning repositories for uncommitted changes…")

	type candidate struct {
		repo      *db.Repository
		protected bool
	}

	var candidates []candidate

	for _, repo := range repos {
		dirty, err := git.IsDirty(repo.Path)
		if err != nil {
			color.Yellow("  ⚠  %s: cannot check status (%v) — skipping", repo.Alias, err)
			continue
		}
		if !dirty {
			continue
		}

		onDefault, err := git.IsDefaultBranch(repo.Path, repo.DefaultBranch)
		if err != nil {
			color.Yellow("  ⚠  %s: cannot detect branch (%v) — treating as unprotected", repo.Alias, err)
			onDefault = false
		}

		candidates = append(candidates, candidate{repo: repo, protected: onDefault})
	}

	if len(candidates) == 0 {
		fmt.Println("No dirty repositories found.")
		return nil
	}

	// Build display slice and disabled indices for MultiSelect.
	displayRepos := make([]*db.Repository, len(candidates))
	disabledIdxs := make([]int, 0)
	for i, c := range candidates {
		displayRepos[i] = c.repo
		if c.protected {
			disabledIdxs = append(disabledIdxs, i)
		}
	}

	// Step 2: Multi-select repos.
	chosen, err := tui.MultiSelect(
		displayRepos,
		"Select repositories to commit",
		false,
		disabledIdxs,
	)
	if err != nil {
		// "no repositories selected" or "cancelled" — not a fatal error.
		fmt.Println(err)
		return nil
	}

	// Step 3: Sequential per-repo commit workflow.
	results := make([]repoCommitResult, 0, len(chosen))

	for _, repo := range chosen {
		fmt.Printf("\n%s\n", color.CyanString("━━━ %s ━━━", repo.Alias))

		// 3a. Get dirty files.
		porcelainLines, err := git.DirtyFilesWithStatus(repo.Path)
		if err != nil {
			color.Red("  ✗ Cannot list dirty files: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}
		if len(porcelainLines) == 0 {
			color.Yellow("  ⚠  No dirty files found (may have been cleaned externally) — skipping")
			results = append(results, repoCommitResult{alias: repo.Alias, skipped: true})
			continue
		}

		// 3b. File picker.
		selectedFiles, err := tui.FileSelect(
			porcelainLines,
			fmt.Sprintf("Select files to stage for %s", repo.Alias),
		)
		if err != nil {
			if err.Error() == "cancelled" {
				color.Yellow("  ⚠  Skipped (cancelled file selection)")
				results = append(results, repoCommitResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ File selection error: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}

		// 3c. Commit message input.
		message, err := tui.CommitMessageInput(repo.Alias)
		if err != nil {
			if err.Error() == "cancelled" {
				color.Yellow("  ⚠  Skipped (cancelled commit message)")
				results = append(results, repoCommitResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ Commit message error: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}

		// 3d. Stage files.
		if err := git.StageFiles(repo.Path, selectedFiles); err != nil {
			color.Red("  ✗ git add failed: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}
		color.Green("  ✓ Staged %d file(s)", len(selectedFiles))

		// 3e. Commit.
		out, err := git.Commit(repo.Path, message)
		if err != nil {
			color.Red("  ✗ git commit failed: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}
		color.Green("  ✓ Committed: %s", firstLine(out))

		// 3f. Push (unless --no-push).
		pushed := false
		if !noPush {
			if err := git.Push(repo.Path); err != nil {
				color.Red("  ✗ git push failed: %v", err)
				results = append(results, repoCommitResult{alias: repo.Alias, err: err})
				continue
			}
			color.Green("  ✓ Pushed")
			pushed = true
		} else {
			color.Yellow("  ⚠  Push skipped (--no-push)")
		}

		results = append(results, repoCommitResult{alias: repo.Alias, pushed: pushed})
	}

	// Step 4: Final summary.
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
			if r.pushed {
				fmt.Printf("  %s  %s (committed + pushed)\n", color.GreenString("✓"), r.alias)
			} else {
				fmt.Printf("  %s  %s (committed)\n", color.GreenString("✓"), r.alias)
			}
		}
	}

	fmt.Println()
	fmt.Printf("%s  %s  %s\n",
		color.GreenString("%d committed", succeeded),
		color.YellowString("%d skipped", skipped),
		color.RedString("%d failed", failed),
	)

	return nil
}

// firstLine returns only the first line of a multi-line string.
func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}

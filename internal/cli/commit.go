package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/db"
	"github.com/alexandreferreira/gitm/internal/git"
)

func commitCmd() *cobra.Command {
	var (
		noPush      bool
		repoAliases []string
	)

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Interactively stage files and commit across dirty repositories",
		Long: `gitm commit scans all registered repositories for uncommitted changes,
lets you pick which repos to commit, then walks through each one sequentially:
  1. Select files to stage
  2. Enter a commit message
  3. Stage selected files, commit, and push (use --no-push to skip push)

Repositories on their default branch are shown but cannot be selected (protected).

Use --repo to target specific repositories by alias, bypassing the interactive
multi-select UI entirely. Non-dirty repos in the list are silently skipped.`,
		Example: `  gitm commit
  gitm commit --no-push
  gitm commit --repo api-gateway,auth-service
  gitm commit --repo api-gateway --no-push`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(noPush, repoAliases)
		},
	}

	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip git push after committing")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated), bypasses interactive selection")

	return cmd
}

// repoCommitResult holds the outcome for a single repo.
type repoCommitResult struct {
	alias   string
	skipped bool
	err     error
	pushed  bool
}

func runCommit(noPush bool, repoAliases []string) error {
	return runCommitWithUI(liveUI{}, noPush, repoAliases)
}

func runCommitWithUI(ui ui, noPush bool, repoAliases []string) error {
	return runCommitWithBranchLookup(ui, noPush, repoAliases, git.CurrentBranch)
}

func runCommitWithBranchLookup(ui ui, noPush bool, repoAliases []string, currentBranch func(string) (string, error)) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
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
		dirty, dirtyErr := git.IsDirty(repo.Path)
		if dirtyErr != nil {
			color.Yellow("  ⚠  %s: cannot check status (%v) — skipping", repo.Alias, dirtyErr)
			continue
		}
		if !dirty {
			continue
		}

		onDefault, branchErr := git.IsDefaultBranch(repo.Path, repo.DefaultBranch)
		if branchErr != nil {
			color.Yellow("  ⚠  %s: cannot detect branch (%v) — treating as unprotected", repo.Alias, branchErr)
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

	// Step 2: Multi-select repos (skipped when --repo is provided).
	var chosen []*db.Repository
	if len(repoAliases) > 0 {
		// --repo bypasses the UI — use all dirty, unprotected candidates directly.
		for _, c := range candidates {
			if !c.protected {
				chosen = append(chosen, c.repo)
			} else {
				color.Yellow("  ⚠  %s: on default branch — skipping (protected)", c.repo.Alias)
			}
		}
		if len(chosen) == 0 {
			fmt.Println("No dirty repositories to commit in the specified repos.")
			return nil
		}
	} else {
		chosen, err = ui.MultiSelect(
			displayRepos,
			"Select repositories to commit",
			false,
			disabledIdxs,
		)
		if err != nil {
			// "no repositories selected" or "canceled" — not a fatal error.
			fmt.Println(err)
			return nil
		}
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
		selectedFiles, err := ui.FileSelect(
			porcelainLines,
			fmt.Sprintf("Select files to stage for %s", repo.Alias),
		)
		if err != nil {
			if err.Error() == "canceled" {
				color.Yellow("  ⚠  Skipped (canceled file selection)")
				results = append(results, repoCommitResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ File selection error: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}

		// 3c. Commit message input.
		branchName, err := currentBranch(repo.Path)
		if err != nil {
			color.Yellow("  ⚠  Cannot detect current branch: %v — continuing without branch prefix", err)
			branchName = ""
		}

		message, err := ui.CommitMessageInput(repo.Alias, branchName)
		if err != nil {
			if err.Error() == "canceled" {
				color.Yellow("  ⚠  Skipped (canceled commit message)")
				results = append(results, repoCommitResult{alias: repo.Alias, skipped: true})
				continue
			}
			color.Red("  ✗ Commit message error: %v", err)
			results = append(results, repoCommitResult{alias: repo.Alias, err: err})
			continue
		}
		if branchName != "" {
			message = branchName + " " + message
		}

		// 3d. Stage files.
		if stageErr := git.StageFiles(repo.Path, selectedFiles); stageErr != nil {
			color.Red("  ✗ git add failed: %v", stageErr)
			results = append(results, repoCommitResult{alias: repo.Alias, err: stageErr})
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

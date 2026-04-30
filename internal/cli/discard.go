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

func discardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discard",
		Short: "Discard uncommitted changes in selected repositories",
		Long: `Interactively select which repositories to discard changes in.
Only repositories with uncommitted changes are shown — if none have changes,
the command exits with a message.

For each selected repository this runs:
  git checkout -- .   (discard modifications to tracked files)
  git clean -fd       (remove untracked files and directories)

WARNING: This operation is irreversible. Discarded changes cannot be recovered.`,
		Example: `  gitm discard`,
		Args:    cobra.NoArgs,
		RunE:    runDiscard,
	}
}

func runDiscard(cmd *cobra.Command, args []string) error {
	return runDiscardWithUI(liveUI{})
}

func runDiscardWithUI(ui ui) error {
	allRepos, err := database.ListRepositories()
	if err != nil {
		return err
	}
	if len(allRepos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	// Filter to only repos that actually have uncommitted changes.
	var dirtyRepos []*db.Repository
	for _, repo := range allRepos {
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

	// Interactive multi-select — only dirty repos are shown.
	chosen, err := ui.MultiSelect(
		dirtyRepos,
		"WARNING: Select repositories to discard changes in (irreversible)",
		false,
		nil,
	)
	if err != nil {
		return err
	}

	fmt.Printf("\nDiscarding changes in %d repository(ies)…\n\n", len(chosen))

	runner.Run(chosen, func(repo *db.Repository) (string, string, error) {
		// Capture file list BEFORE discarding so we can report what was removed.
		files, filesErr := git.DirtyFiles(repo.Path)
		if filesErr != nil {
			return "", "", fmt.Errorf("list dirty files: %w", filesErr)
		}

		if err := git.DiscardChanges(repo.Path); err != nil {
			return "", "", err
		}

		// Build a multi-line message: summary line + each file indented below.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("discarded %d file(s):\n", len(files)))
		for _, f := range files {
			sb.WriteString(fmt.Sprintf("       %s\n", color.New(color.FgWhite).Sprint(f)))
		}
		return strings.TrimRight(sb.String(), "\n"), "", nil
	})

	return nil
}

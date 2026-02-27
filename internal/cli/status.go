package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/git"
)

func statusCmd() *cobra.Command {
	var fetchRemote bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the git status of all registered repositories",
		Long: `Display a summary table of all repositories showing:
  - Current branch
  - Whether the working tree is dirty (has uncommitted changes)
  - How many commits ahead/behind origin (based on last known remote state)

Use --fetch to run git fetch on all repos first for up-to-date remote numbers.
Without --fetch the command is near-instant (no network calls).`,
		Example: `  gitm status
  gitm status --fetch`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args, fetchRemote)
		},
	}

	cmd.Flags().BoolVar(&fetchRemote, "fetch", false, "Fetch from origin before checking ahead/behind (slower but accurate)")
	return cmd
}

type repoStatus struct {
	name   string
	branch string
	dirty  string
	ahead  int
	behind int
	err    string
}

func runStatus(cmd *cobra.Command, args []string, fetchRemote bool) error {
	repos, err := database.ListRepositories()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	if fetchRemote {
		fmt.Printf("Fetching from origin and collecting status for %d repositories…\n\n", len(repos))
	} else {
		fmt.Printf("Collecting status for %d repositories…\n\n", len(repos))
	}

	// Collect status in parallel.
	statuses := make([]repoStatus, len(repos))
	done := make(chan struct{}, len(repos))

	// Use a semaphore to limit concurrency.
	sem := make(chan struct{}, 10)

	for i, repo := range repos {
		i, repo := i, repo
		go func() {
			sem <- struct{}{}
			defer func() {
				<-sem
				done <- struct{}{}
			}()

			s := repoStatus{name: repo.Name}

			branch, err := git.CurrentBranch(repo.Path)
			if err != nil {
				s.err = err.Error()
				statuses[i] = s
				return
			}
			s.branch = branch

			isDirty, err := git.IsDirty(repo.Path)
			if err != nil {
				s.err = err.Error()
				statuses[i] = s
				return
			}
			if isDirty {
				files, _ := git.DirtyFiles(repo.Path)
				s.dirty = fmt.Sprintf("%d modified", len(files))
			} else {
				s.dirty = "clean"
			}

			s.ahead, s.behind, _ = git.AheadBehind(repo.Path, fetchRemote)
			statuses[i] = s
		}()
	}

	// Wait for all goroutines.
	for range repos {
		<-done
	}

	printStatusTable(statuses)
	return nil
}

func printStatusTable(statuses []repoStatus) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	dim := color.New(color.FgWhite)

	fmt.Printf("%-22s  %-24s  %-12s  %s\n",
		header.Sprint("REPO"),
		header.Sprint("BRANCH"),
		header.Sprint("DIRTY"),
		header.Sprint("REMOTE"),
	)
	fmt.Println(strings.Repeat("─", 80))

	for _, s := range statuses {
		if s.err != "" {
			fmt.Printf("%-22s  %s\n", cyan.Sprint(s.name), red.Sprintf("ERROR: %s", s.err))
			continue
		}

		dirtyCol := green.Sprint(s.dirty)
		if s.dirty != "clean" {
			dirtyCol = yellow.Sprint(s.dirty)
		}

		remoteCol := green.Sprint("up to date")
		if s.behind > 0 && s.ahead > 0 {
			remoteCol = yellow.Sprintf("↓%d ↑%d", s.behind, s.ahead)
		} else if s.behind > 0 {
			remoteCol = yellow.Sprintf("%d behind", s.behind)
		} else if s.ahead > 0 {
			remoteCol = dim.Sprintf("%d ahead", s.ahead)
		}

		fmt.Printf("%-22s  %-24s  %-12s  %s\n",
			cyan.Sprint(s.name),
			s.branch,
			dirtyCol,
			remoteCol,
		)
	}
}

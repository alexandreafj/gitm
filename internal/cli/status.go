package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/git"
)

func statusCmd() *cobra.Command {
	var (
		fetchRemote bool
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the git status of all registered repositories",
		Long: `Display a summary table of all repositories showing:
  - Current branch
  - Whether the working tree is dirty (has uncommitted changes)
  - How many commits ahead/behind origin (based on last known remote state)

Use --fetch to run git fetch on all repos first for up-to-date remote numbers.
Without --fetch the command is near-instant (no network calls).

Use --repo / -r to limit output to specific repositories by alias.
Use --group / -g to limit output to repositories in a group.
When both are provided, gitm shows only aliases that also belong to the group.`,
		Example: `  gitm status
  gitm status --fetch
  gitm status -r api-gateway
  gitm status -g backend
  gitm status -r api-gateway,auth-service -g backend --fetch`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runStatus(fetchRemote, repoAliases)
			}
			return runStatusWithGroup(fetchRemote, repoAliases, groupName)
		},
	}

	cmd.Flags().BoolVar(&fetchRemote, "fetch", false, "Fetch from origin before checking ahead/behind (slower but accurate)")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &groupName)
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

func runStatus(fetchRemote bool, repoAliases []string) error {
	return runStatusWithGroup(fetchRemote, repoAliases, "")
}

func runStatusWithGroup(fetchRemote bool, repoAliases []string, groupName string) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
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

			s := repoStatus{name: repo.Alias}

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
				files, filesErr := git.DirtyFiles(repo.Path)
				if filesErr != nil {
					s.err = filesErr.Error()
					statuses[i] = s
					return
				}
				s.dirty = fmt.Sprintf("%d changed", len(files))
			} else {
				s.dirty = "clean"
			}

			ahead, behind, abErr := git.AheadBehind(repo.Path, fetchRemote)
			if abErr != nil {
				s.err = abErr.Error()
				statuses[i] = s
				return
			}
			s.ahead, s.behind = ahead, behind
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

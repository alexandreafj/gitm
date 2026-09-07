package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/runner"
)

func statusCmd() *cobra.Command {
	var options statusOptions
	var repoAliases []string
	var groupName string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository status and highlight work needing attention",
		Long: `Display each selected repository's branch, changed files, and upstream state.

Without --fetch, counts use cached remote-tracking refs and no network calls
are made. Use --fetch to refresh them. Failed refreshes are reported alongside
any cached counts, and the command exits nonzero if any repository fails.
A branch without an upstream is shown explicitly; unknown counts are never zero.

Use --attention to show only repositories with changes, ahead/behind commits,
conflicts, unfinished Git operations, detached or unborn HEAD, missing upstream,
or inspection errors. Normal states such as dirty files do not cause failure.

Use --json for a versioned JSON report, with no progress text or ANSI colors on
stdout. Unavailable dirty/count values are null; repositories is always an array.
Errors are retained in the report and also cause a nonzero exit status.

Use --repo / -r or --group / -g to narrow the active context's repositories.
When both are supplied, only aliases within the group are included.`,
		Example: `  gitm status
  gitm status --fetch
  gitm status --attention -g backend
  gitm status --json
  gitm status --attention --json -r api-gateway,auth-service`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusWithOptions(cmd.OutOrStdout(), repoAliases, groupName, options)
		},
	}
	cmd.Flags().BoolVar(&options.fetch, "fetch", false, "Fetch before inspection; report failed refreshes and stale counts")
	cmd.Flags().BoolVar(&options.attention, "attention", false, "Show only repositories needing attention, including errors")
	cmd.Flags().BoolVar(&options.json, "json", false, "Write a versioned JSON report to stdout")
	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to repository aliases (comma-separated)")
	addGroupFlag(cmd, &groupName)
	return cmd
}

type statusOptions struct {
	fetch     bool
	attention bool
	json      bool
}

type statusReport struct {
	Version      int          `json:"version"`
	Context      string       `json:"context"`
	Repositories []repoStatus `json:"repositories"`
}

func runStatus(fetchRemote bool, repoAliases []string) error {
	return runStatusWithGroup(fetchRemote, repoAliases, "")
}

func runStatusWithGroup(fetchRemote bool, repoAliases []string, groupName string) error {
	return runStatusWithOptions(os.Stdout, repoAliases, groupName, statusOptions{fetch: fetchRemote})
}

func runStatusWithOptions(w io.Writer, repoAliases []string, groupName string, options statusOptions) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	active, err := database.ActiveContext()
	if err != nil {
		return fmt.Errorf("read status context: %w", err)
	}
	if len(repos) == 0 && !options.json {
		if _, err := fmt.Fprintln(w, noReposMessage(repoAliases, groupName)); err != nil {
			return fmt.Errorf("write empty status: %w", err)
		}
		return nil
	}
	return writeStatusReport(w, active.Name, repos, options)
}

func writeStatusReport(w io.Writer, contextName string, repos []*db.Repository, options statusOptions) error {
	statuses := make([]repoStatus, len(repos))
	indices := make(map[int64]int, len(repos))
	for i, repo := range repos {
		indices[repo.ID] = i
	}
	runner.RunEach(repos, func(repo *db.Repository) (string, string, error) {
		statuses[indices[repo.ID]] = collectRepoStatus(repo, options.fetch)
		return "", "", nil
	}, nil)

	report := statusReport{Version: 1, Context: contextName, Repositories: make([]repoStatus, 0, len(repos))}
	var failed int
	for _, status := range statuses {
		if status.Error != "" {
			failed++
		}
		if !options.attention || status.needsAttention() {
			report.Repositories = append(report.Repositories, status)
		}
	}
	if options.json {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("write status JSON: %w", err)
		}
	} else {
		table, err := renderStatusTable(report)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, table); err != nil {
			return fmt.Errorf("write status table: %w", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("status inspection failed for %d repository(ies)", failed)
	}
	return nil
}

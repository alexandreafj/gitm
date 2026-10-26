package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

type fetchOptions struct {
	prune       bool
	repoAliases []string
	groupName   string
}

func fetchCmd() *cobra.Command {
	var opts fetchOptions

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch from origin across repositories in parallel",
		Long: `Run git fetch in every selected repository, in parallel.

Only remote-tracking refs (origin/*) change. The working tree, the index,
HEAD and your local branches are never touched, so it is safe to run at any
time, even with uncommitted changes. Afterwards, gitm status and
gitm branches show fresh ahead/behind numbers without their own --fetch.

Use --prune to also delete remote-tracking refs whose branch was deleted on
origin (git fetch --prune). Local branches are never deleted; see gitm prune
for that.

Each repository gets one row: ok, or the error. A repository without an
origin remote, or whose origin cannot be reached, is an error. The other
repositories still run, and the command then exits nonzero.

Use --repo / -r to limit to specific repositories by alias. Use --group / -g
to limit to repositories in a group. When both are given, only aliases that
also belong to the group are fetched.`,
		Example: `  gitm fetch
  gitm fetch --prune
  gitm fetch -r api-gateway,auth-service
  gitm fetch -g backend --prune`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(os.Stdout, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.prune, "prune", false, "Also delete remote-tracking refs whose branch was deleted on origin")
	cmd.Flags().StringSliceVarP(&opts.repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &opts.groupName)
	return cmd
}

func runFetch(w io.Writer, opts fetchOptions) error {
	repos, err := resolveReposWithGroup(opts.repoAliases, opts.groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Fprintln(w, noReposMessage(opts.repoAliases, opts.groupName))
		return nil
	}

	fmt.Fprintf(w, "Fetching from origin for %d repositories…\n\n", len(repos))
	results := runner.RunEach(repos, func(repo *db.Repository) (string, string, error) {
		return "", "", fetchOrigin(repo.Path, opts.prune)
	}, nil)

	tw := newTable(w)
	headerRow(tw, "REPO", "RESULT")
	for _, r := range results {
		result := cell(okColor, "ok")
		if r.Err != nil {
			result = cell(errColor, "ERROR: "+oneLine(r.Err))
		}
		row(tw, cell(aliasColor, r.Repo.Alias), result)
	}
	if err := flushTable(tw, "fetch"); err != nil {
		return err
	}
	runner.FprintSummary(w, results)

	if n := runner.ErrorCount(results); n > 0 {
		return fmt.Errorf("fetch failed for %d repository(ies)", n)
	}
	return nil
}

// fetchOrigin checks for origin first: a plain git fetch with no remote
// configured exits 0 without doing anything, which would read as "ok".
func fetchOrigin(path string, prune bool) error {
	ok, err := git.RemoteConfigured(path, "origin")
	if err != nil {
		return fmt.Errorf("check origin remote: %w", err)
	}
	if !ok {
		return errors.New("no origin remote configured")
	}
	if prune {
		err = git.FetchPrune(path)
	} else {
		err = git.Fetch(path)
	}
	if err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

// oneLine collapses a multi-line git error (exit status plus stderr) so it
// fits in a single table cell or output line.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

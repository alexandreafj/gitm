package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

type logOptions struct {
	since         string
	until         string
	author        string
	allAuthors    bool
	currentBranch bool
	maxCount      int
	repoAliases   []string
	groupName     string
}

func logCmd() *cobra.Command {
	var opts logOptions

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show your recent commits across repositories (standup helper)",
		Long: `Show the commits you made recently, across every selected repository.

The question it answers: what did I touch yesterday, across everything?

By default it lists commits since yesterday, by you, on all local branches,
grouped by repository. "You" is the user.email from each repository's git
config, or user.name when no email is set. When neither is set, that
repository shows commits by all authors and a warning names it.

  --since / --until   Any date git understands: "yesterday", "last friday",
                      "3 days ago", "2026-10-20". Passed to git as is.
  --author            A git --author pattern, matched against "Name <email>".
                      Replaces the user.email default.
  --all-authors       Commits by everyone.
  --current-branch    Only the checked-out branch, not all local branches.
  --max-count / -n    At most N commits per repository (0 = no limit).

Each commit shows its short hash, relative commit date and subject, newest
first. Repositories with no matching commits are left out. If no repository
has any, it says so and exits 0.

Read-only: it runs one git log per repository, in parallel, and never changes
anything. Output is in alias order. A repository that fails (missing
directory, git error) shows an error entry; the others still print and the
command exits nonzero.

Use --repo / -r to limit to specific repositories by alias. Use --group / -g
to limit to repositories in a group. When both are given, only aliases that
also belong to the group are shown.`,
		Example: `  gitm log
  gitm log --since "last friday"
  gitm log --since "2026-10-20" --until "2026-10-21"
  gitm log --all-authors -g backend
  gitm log --author "jane@example.com" -n 5
  gitm log --current-branch -r api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(os.Stdout, opts)
		},
	}

	cmd.Flags().StringVar(&opts.since, "since", "yesterday", "Show commits more recent than this date (any date git understands)")
	cmd.Flags().StringVar(&opts.until, "until", "", "Show commits older than this date")
	cmd.Flags().StringVar(&opts.author, "author", "", "Only commits whose author matches this pattern (default: your user.email)")
	cmd.Flags().BoolVar(&opts.allAuthors, "all-authors", false, "Show commits by every author")
	cmd.Flags().BoolVar(&opts.currentBranch, "current-branch", false, "Only the checked-out branch instead of all local branches")
	cmd.Flags().IntVarP(&opts.maxCount, "max-count", "n", 0, "At most this many commits per repository (0 = no limit)")
	cmd.Flags().StringSliceVarP(&opts.repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &opts.groupName)
	cmd.MarkFlagsMutuallyExclusive("author", "all-authors")
	return cmd
}

type repoLog struct {
	repo    *db.Repository
	commits []git.LogEntry
	warning string
	err     error
}

func runLog(w io.Writer, opts logOptions) error {
	if opts.maxCount < 0 {
		return fmt.Errorf("--max-count must be 0 or more, got %d", opts.maxCount)
	}
	repos, err := resolveReposWithGroup(opts.repoAliases, opts.groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Fprintln(w, noReposMessage(opts.repoAliases, opts.groupName))
		return nil
	}
	slices.SortFunc(repos, func(a, b *db.Repository) int { return strings.Compare(a.Alias, b.Alias) })

	logs := make([]repoLog, len(repos))
	index := make(map[int64]int, len(repos))
	for i, repo := range repos {
		index[repo.ID] = i
	}
	runner.RunEach(repos, func(repo *db.Repository) (string, string, error) {
		logs[index[repo.ID]] = collectLog(repo, opts)
		return "", "", nil
	}, nil)

	for _, l := range logs {
		if l.warning != "" {
			fmt.Fprintf(w, "Warning: %s: %s\n", l.repo.Alias, l.warning)
		}
	}

	commits, shown, failed := 0, 0, 0
	for _, l := range logs {
		if l.err == nil && len(l.commits) == 0 {
			continue
		}
		if shown+failed > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, cell(aliasColor, l.repo.Alias))
		if l.err != nil {
			failed++
			fmt.Fprintf(w, "  %s\n", cell(errColor, "ERROR: "+oneLine(l.err)))
			continue
		}
		tw := newTable(w)
		for _, c := range l.commits {
			row(tw, "  "+cell(warnColor, c.Hash), cell(dimColor, c.Date), c.Subject)
		}
		if err := flushTable(tw, "log"); err != nil {
			return err
		}
		commits += len(l.commits)
		shown++
	}

	if shown+failed > 0 {
		fmt.Fprintln(w)
	}
	if commits == 0 {
		fmt.Fprintln(w, "No commits found.")
	} else {
		fmt.Fprintf(w, "%d commit(s) across %d repository(ies).\n", commits, shown)
	}

	if failed > 0 {
		return fmt.Errorf("log failed for %d repository(ies)", failed)
	}
	return nil
}

func collectLog(repo *db.Repository, opts logOptions) repoLog {
	l := repoLog{repo: repo}
	author := opts.author
	if author == "" && !opts.allAuthors {
		var err error
		author, err = currentAuthor(repo.Path)
		if err != nil {
			l.err = err
			return l
		}
		if author == "" {
			l.warning = "no user.email or user.name configured, showing commits by all authors"
		}
	}

	commits, err := git.Log(repo.Path, git.LogOptions{
		Since:       opts.since,
		Until:       opts.until,
		Author:      author,
		AllBranches: !opts.currentBranch,
		MaxCount:    opts.maxCount,
	})
	if err != nil {
		l.err = fmt.Errorf("git log: %w", err)
		return l
	}
	l.commits = commits
	return l
}

// currentAuthor returns a git --author pattern for the repository's user.
// The email is wrapped in <> so bob@x.com does not also match jimbob@x.com.
func currentAuthor(path string) (string, error) {
	email, err := git.ConfigValue(path, "user.email")
	if err != nil {
		return "", fmt.Errorf("read user.email: %w", err)
	}
	if email != "" {
		return "<" + email + ">", nil
	}
	name, err := git.ConfigValue(path, "user.name")
	if err != nil {
		return "", fmt.Errorf("read user.name: %w", err)
	}
	return name, nil
}

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
	"github.com/alexandreafj/gitm/internal/runner"
)

type doctorSeverity int

const (
	doctorWarning doctorSeverity = iota
	doctorError
)

type doctorCheck struct {
	severity doctorSeverity
	message  string
}

type doctorReport struct {
	repo   *db.Repository
	checks []doctorCheck
}

func (r doctorReport) hasErrors() bool {
	for _, check := range r.checks {
		if check.severity == doctorError {
			return true
		}
	}
	return false
}

func (r doctorReport) hasWarnings() bool {
	for _, check := range r.checks {
		if check.severity == doctorWarning {
			return true
		}
	}
	return false
}

func doctorCmd() *cobra.Command {
	var (
		repoAliases []string
		groupName   string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check registered repositories for common health issues",
		Long: `Run worktree-read-only diagnostics across registered repositories.

gitm doctor checks whether each repository path still exists, is still a git
repository, has an origin remote, is on a readable branch, has an upstream, has
its detected default branch locally, has uncommitted changes, or is in the
middle of a merge/rebase/cherry-pick/revert/bisect operation.

Before these checks, gitm asks origin for each repository's live default branch
through a lightweight network lookup of its symbolic HEAD (not a full fetch).
Unlike other commands, doctor does not trust the local origin/HEAD, which stays
stale after the remote renames its default branch. A changed default is
recorded in both GitM's SQLite cache and origin/HEAD, with no fetch required.
Other commands then pick it up from origin/HEAD, consulting origin once per run
until the new branch has been fetched. A failed lookup emits a warning and uses
the cached value. Apart from that origin/HEAD record, doctor never fetches
remote refs or mutates the repository's worktree or Git metadata.

Warnings call out normal conditions that may need attention, such as dirty
working trees or missing upstreams. Errors are reserved for broken registrations
or repositories that cannot be inspected.

Use --repo / -r to limit diagnostics to specific repositories by alias.
Use --group / -g to limit diagnostics to repositories in a group.
When both are provided, gitm checks only aliases that also belong to the group.`,
		Example: `  gitm doctor
  gitm doctor --group backend
  gitm doctor --repo api-gateway
  gitm doctor -r api-gateway,auth-service -g backend`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupName == "" {
				return runDoctor(repoAliases)
			}
			return runDoctorWithGroup(repoAliases, groupName)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	addGroupFlag(cmd, &groupName)
	return cmd
}

func runDoctor(repoAliases []string) error {
	return runDoctorWithGroup(repoAliases, "")
}

func runDoctorWithGroup(repoAliases []string, groupName string) error {
	repos, err := resolveReposWithGroup(repoAliases, groupName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println(noReposMessage(repoAliases, groupName))
		return nil
	}
	// A stale origin/HEAD survives fetch and prune, so doctor asks origin
	// directly to notice a renamed default.
	recheck := func(repo *db.Repository) (string, error) {
		local, err := git.LocalDefaultBranch(repo.Path)
		if err != nil {
			// An unreadable origin/HEAD is rewritten with the live answer.
			local = ""
		}
		return liveDefaultBranch(repo.Path, local, true)
	}
	if err := reconcileDefaultBranchesWith(database, repos, true, recheck); err != nil {
		return fmt.Errorf("refresh default branches: %w", err)
	}

	fmt.Printf("Checking %d registered repository(ies)…\n\n", len(repos))

	reports := runner.Collect(repos, func(repo *db.Repository) doctorReport {
		return inspectRepoHealth(repo)
	})

	if err := printDoctorReports(os.Stdout, reports); err != nil {
		return err
	}

	var errorCount int
	for _, report := range reports {
		if report.hasErrors() {
			errorCount++
		}
	}
	if errorCount > 0 {
		return fmt.Errorf("repository health check failed: %d repository(ies) need attention", errorCount)
	}
	return nil
}

func inspectRepoHealth(repo *db.Repository) doctorReport {
	report := doctorReport{repo: repo}

	info, err := os.Stat(repo.Path)
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorError,
			message:  fmt.Sprintf("path is not accessible: %v", err),
		})
		return report
	}
	if !info.IsDir() {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorError,
			message:  "path is not a directory",
		})
		return report
	}
	if !git.IsGitRepo(repo.Path) {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorError,
			message:  "path is not a git repository root",
		})
		return report
	}

	branch, err := git.CurrentBranch(repo.Path)
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorError,
			message:  fmt.Sprintf("cannot read current branch: %v", err),
		})
	} else if branch == "HEAD" {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  "detached HEAD",
		})
	}

	if !git.BranchExists(repo.Path, repo.DefaultBranch) {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("default branch %q is not present locally", repo.DefaultBranch),
		})
	}

	origin, err := git.RemoteConfigured(repo.Path, "origin")
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("cannot check origin remote: %v", err),
		})
	} else if !origin {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  "origin remote is not configured",
		})
	}

	upstream, err := git.HasUpstream(repo.Path)
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("cannot check upstream: %v", err),
		})
	} else if !upstream {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  "current branch has no upstream",
		})
	}

	dirty, err := git.IsDirty(repo.Path)
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("cannot check working tree: %v", err),
		})
	} else if dirty {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  "working tree has uncommitted changes",
		})
	}

	ops, err := git.InProgressOperations(repo.Path)
	if err != nil {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("cannot check in-progress operations: %v", err),
		})
	} else if len(ops) > 0 {
		report.checks = append(report.checks, doctorCheck{
			severity: doctorWarning,
			message:  fmt.Sprintf("git operation in progress: %s", strings.Join(ops, ", ")),
		})
	}

	return report
}

func printDoctorReports(w io.Writer, reports []doctorReport) error {
	tw := newTable(w)
	headerRow(tw, "REPO", "STATUS", "DETAILS")

	for _, report := range reports {
		status := cell(okColor, "OK")
		details := "healthy"
		if report.hasErrors() {
			status = cell(errColor, "ERROR")
		} else if report.hasWarnings() {
			status = cell(warnColor, "WARN")
		}

		if len(report.checks) > 0 {
			messages := make([]string, 0, len(report.checks))
			for _, check := range report.checks {
				messages = append(messages, check.message)
			}
			details = strings.Join(messages, "; ")
		}

		row(tw, cell(aliasColor, report.repo.Alias), status, details)
	}
	return flushTable(tw, "doctor")
}

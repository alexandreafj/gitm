package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
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
	var repoAliases []string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check registered repositories for common health issues",
		Long: `Run read-only diagnostics across registered repositories.

gitm doctor checks whether each repository path still exists, is still a git
repository, has an origin remote, is on a readable branch, has an upstream, has
its configured default branch locally, has uncommitted changes, or is in the
middle of a merge/rebase/cherry-pick/revert/bisect operation.

Warnings call out normal conditions that may need attention, such as dirty
working trees or missing upstreams. Errors are reserved for broken registrations
or repositories that cannot be inspected.

Use --repo / -r to limit diagnostics to specific repositories by alias.`,
		Example: `  gitm doctor
  gitm doctor --repo api-gateway
  gitm doctor -r api-gateway,auth-service`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(repoAliases)
		},
	}

	cmd.Flags().StringSliceVarP(&repoAliases, "repo", "r", nil, "Limit to specific repository aliases (comma-separated)")
	return cmd
}

func runDoctor(repoAliases []string) error {
	repos, err := resolveRepos(repoAliases)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories registered. Run `gitm repo add <path>` to add one.")
		return nil
	}

	fmt.Printf("Checking %d registered repository(ies)…\n\n", len(repos))

	reports := make([]doctorReport, len(repos))
	done := make(chan struct{}, len(repos))
	sem := make(chan struct{}, 10)

	for i, repo := range repos {
		i, repo := i, repo
		go func() {
			sem <- struct{}{}
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			reports[i] = inspectRepoHealth(repo)
		}()
	}

	for range repos {
		<-done
	}

	printDoctorReports(reports)

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

func printDoctorReports(reports []doctorReport) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	fmt.Printf("%-22s  %-7s  %s\n",
		header.Sprint("REPO"),
		header.Sprint("STATUS"),
		header.Sprint("DETAILS"),
	)
	fmt.Println(strings.Repeat("─", 90))

	for _, report := range reports {
		status := green.Sprint("OK")
		details := "healthy"
		if report.hasErrors() {
			status = red.Sprint("ERROR")
		} else if report.hasWarnings() {
			status = yellow.Sprint("WARN")
		}

		if len(report.checks) > 0 {
			messages := make([]string, 0, len(report.checks))
			for _, check := range report.checks {
				messages = append(messages, check.message)
			}
			details = strings.Join(messages, "; ")
		}

		fmt.Printf("%-22s  %-7s  %s\n",
			cyan.Sprint(report.repo.Alias),
			status,
			details,
		)
	}
}

// Package runner provides a parallel execution engine for multi-repo git operations.
package runner

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	"golang.org/x/sync/errgroup"

	"github.com/alexandreferreira/gitm/internal/db"
)

// Status represents the outcome of an operation on a single repository.
type Status int

const (
	StatusSuccess Status = iota
	StatusSkipped
	StatusError
)

// Result holds the outcome of running an operation against one repository.
type Result struct {
	Repo    *db.Repository
	Status  Status
	Message string
	Err     error
}

// OpFunc is the function signature for a repository operation.
// Return (message, skip reason, error). If skipReason != "" the result is Skipped.
type OpFunc func(repo *db.Repository) (message string, skipReason string, err error)

// maxConcurrency is the default number of parallel git operations.
const maxConcurrency = 10

var (
	mu     sync.Mutex
	green  = color.New(color.FgGreen, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	red    = color.New(color.FgRed, color.Bold)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
)

// Run executes op against each repo in parallel, streaming results to stdout.
// It returns the collected results after all operations complete.
func Run(repos []*db.Repository, op OpFunc) []Result {
	results := make([]Result, len(repos))
	sem := make(chan struct{}, maxConcurrency)

	var eg errgroup.Group

	for i, repo := range repos {
		i, repo := i, repo // capture loop vars
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			msg, skip, err := op(repo)

			r := Result{Repo: repo}
			switch {
			case err != nil:
				r.Status = StatusError
				r.Message = err.Error()
				r.Err = err
			case skip != "":
				r.Status = StatusSkipped
				r.Message = skip
			default:
				r.Status = StatusSuccess
				r.Message = msg
			}

			results[i] = r
			printResult(r)
			return nil
		})
	}

	_ = eg.Wait()
	printSummary(results)
	return results
}

// printResult prints a result in a thread-safe way.
// If the message contains newlines, the first line is printed with the status
// icon and the remaining lines are indented below it.
func printResult(r Result) {
	mu.Lock()
	defer mu.Unlock()

	label := fmt.Sprintf("[%-20s]", r.Repo.Alias)

	var icon string
	switch r.Status {
	case StatusSuccess:
		icon = green.Sprint("✓")
	case StatusSkipped:
		icon = yellow.Sprint("⚠ SKIPPED:")
	case StatusError:
		icon = red.Sprint("✗ ERROR:")
	}

	lines := strings.SplitN(r.Message, "\n", 2)
	firstLine := lines[0]
	fmt.Printf("%s %s %s\n", cyan.Sprint(label), icon, firstLine)

	// Print any additional lines (e.g. file list) indented under the first.
	if len(lines) == 2 {
		for _, extra := range strings.Split(lines[1], "\n") {
			if extra != "" {
				fmt.Printf("  %s\n", extra)
			}
		}
	}
}

// printSummary prints a summary line after all operations.
func printSummary(results []Result) {
	var success, skipped, errored int
	for _, r := range results {
		switch r.Status {
		case StatusSuccess:
			success++
		case StatusSkipped:
			skipped++
		case StatusError:
			errored++
		}
	}

	parts := []string{
		green.Sprintf("%d succeeded", success),
	}
	if skipped > 0 {
		parts = append(parts, yellow.Sprintf("%d skipped", skipped))
	}
	if errored > 0 {
		parts = append(parts, red.Sprintf("%d failed", errored))
	}

	fmt.Printf("\n%s %s\n", bold.Sprint("Done:"), strings.Join(parts, ", "))
}

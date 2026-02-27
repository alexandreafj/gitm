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

// printResult prints a single result line in a thread-safe way.
func printResult(r Result) {
	mu.Lock()
	defer mu.Unlock()

	label := fmt.Sprintf("[%-20s]", r.Repo.Name)

	switch r.Status {
	case StatusSuccess:
		fmt.Printf("%s %s %s\n", cyan.Sprint(label), green.Sprint("✓"), r.Message)
	case StatusSkipped:
		fmt.Printf("%s %s %s\n", cyan.Sprint(label), yellow.Sprint("⚠ SKIPPED:"), r.Message)
	case StatusError:
		fmt.Printf("%s %s %s\n", cyan.Sprint(label), red.Sprint("✗ ERROR:"), r.Message)
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

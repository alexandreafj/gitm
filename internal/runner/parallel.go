// Package runner provides a parallel execution engine for multi-repo git operations.
package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"golang.org/x/sync/errgroup"

	"github.com/alexandreafj/gitm/internal/db"
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
	green  = color.New(color.FgGreen, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	red    = color.New(color.FgRed, color.Bold)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
)

// Run executes op against each repo in parallel, streaming results to stdout.
// It returns the collected results after all operations complete.
func Run(repos []*db.Repository, op OpFunc) []Result {
	results := RunEach(repos, op, func(result Result) {
		FprintResult(os.Stdout, result)
	})
	FprintSummary(os.Stdout, results)
	return results
}

// RunEach executes op against each repository in parallel and calls onResult
// serially as operations finish. It does not print a summary.
func RunEach(repos []*db.Repository, op OpFunc, onResult func(Result)) []Result {
	results := make([]Result, len(repos))
	sem := make(chan struct{}, maxConcurrency)
	var callbackMu sync.Mutex

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
			if onResult != nil {
				callbackMu.Lock()
				onResult(r)
				callbackMu.Unlock()
			}
			return nil
		})
	}

	//nolint:errcheck // errgroup.Wait() can only error from child goroutines; we handle all errors there
	_ = eg.Wait()
	return results
}

// FprintResult prints a result.
// If the message contains newlines, the first line is printed with the status
// icon and the remaining lines are indented below it.
func FprintResult(w io.Writer, r Result) {
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
	fmt.Fprintf(w, "%s %s %s\n", cyan.Sprint(label), icon, firstLine)

	// Print any additional lines (e.g. file list) indented under the first.
	if len(lines) == 2 {
		for _, extra := range strings.Split(lines[1], "\n") {
			if extra != "" {
				fmt.Fprintf(w, "  %s\n", extra)
			}
		}
	}
}

// FprintSummary prints a summary line after all operations.
func FprintSummary(w io.Writer, results []Result) {
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

	fmt.Fprintf(w, "\n%s %s\n", bold.Sprint("Done:"), strings.Join(parts, ", "))
}

// HasErrors returns true if any result in the slice has StatusError.
func HasErrors(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of results with StatusError.
func ErrorCount(results []Result) int {
	n := 0
	for _, r := range results {
		if r.Status == StatusError {
			n++
		}
	}
	return n
}

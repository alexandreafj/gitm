package runner_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/runner"
)

// newTestRepo creates a test repository with a given alias.
func newTestRepo(alias string) *db.Repository {
	return &db.Repository{
		ID:            int64(len(alias)), // Use length as a simple unique ID
		Name:          alias,
		Alias:         alias,
		Path:          "/test/" + alias,
		DefaultBranch: "main",
	}
}

func TestRunSuccess(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("repo1"),
		newTestRepo("repo2"),
		newTestRepo("repo3"),
	}

	var successCount int
	op := func(repo *db.Repository) (string, string, error) {
		return fmt.Sprintf("processed %s", repo.Alias), "", nil
	}

	results := runner.Run(repos, op)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Status != runner.StatusSuccess {
			t.Errorf("results[%d].Status = %v, want StatusSuccess", i, r.Status)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
		if r.Message == "" {
			t.Errorf("results[%d].Message should not be empty", i)
		}
		successCount++
	}

	if successCount != 3 {
		t.Errorf("expected all 3 repos to succeed, got %d", successCount)
	}
}

func TestRunWithError(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("repo1"),
		newTestRepo("repo2_error"),
		newTestRepo("repo3"),
	}

	op := func(repo *db.Repository) (string, string, error) {
		if repo.Alias == "repo2_error" {
			return "", "", fmt.Errorf("simulated error")
		}
		return "success", "", nil
	}

	results := runner.Run(repos, op)

	errorFound := false
	for i, r := range results {
		if r.Repo.Alias == "repo2_error" {
			if r.Status != runner.StatusError {
				t.Errorf("results[%d].Status = %v, want StatusError", i, r.Status)
			}
			if r.Err == nil {
				t.Errorf("results[%d].Err should not be nil", i)
			}
			errorFound = true
		} else {
			if r.Status != runner.StatusSuccess {
				t.Errorf("results[%d].Status = %v, want StatusSuccess", i, r.Status)
			}
		}
	}

	if !errorFound {
		t.Error("expected to find error result for repo2_error")
	}
}

func TestRunSkipped(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("repo1"),
		newTestRepo("repo2_skip"),
		newTestRepo("repo3"),
	}

	op := func(repo *db.Repository) (string, string, error) {
		if repo.Alias == "repo2_skip" {
			return "", "repository is locked", nil
		}
		return "success", "", nil
	}

	results := runner.Run(repos, op)

	skipFound := false
	for i, r := range results {
		if r.Repo.Alias == "repo2_skip" {
			if r.Status != runner.StatusSkipped {
				t.Errorf("results[%d].Status = %v, want StatusSkipped", i, r.Status)
			}
			if r.Message != "repository is locked" {
				t.Errorf("results[%d].Message = %q, want \"repository is locked\"", i, r.Message)
			}
			skipFound = true
		} else {
			if r.Status != runner.StatusSuccess {
				t.Errorf("results[%d].Status = %v, want StatusSuccess", i, r.Status)
			}
		}
	}

	if !skipFound {
		t.Error("expected to find skipped result for repo2_skip")
	}
}

func TestRunParallelExecution(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("repo1"),
		newTestRepo("repo2"),
		newTestRepo("repo3"),
		newTestRepo("repo4"),
		newTestRepo("repo5"),
	}

	var (
		mu         sync.Mutex
		concurrent int
		maxConcur  int
	)

	op := func(repo *db.Repository) (string, string, error) {
		mu.Lock()
		concurrent++
		if concurrent > maxConcur {
			maxConcur = concurrent
		}
		mu.Unlock()

		// Sleep long enough for goroutines to genuinely overlap
		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		concurrent--
		mu.Unlock()

		return "done", "", nil
	}

	_ = runner.Run(repos, op)

	if maxConcur < 2 {
		t.Errorf("expected parallel execution (maxConcur > 1), got %d", maxConcur)
	}
}

func TestRunMaxConcurrency(t *testing.T) {
	repos := make([]*db.Repository, 20)
	for i := 0; i < 20; i++ {
		repos[i] = newTestRepo(fmt.Sprintf("repo%d", i))
	}

	var (
		mu         sync.Mutex
		concurrent int
		maxConcur  int
	)

	op := func(repo *db.Repository) (string, string, error) {
		mu.Lock()
		concurrent++
		if concurrent > maxConcur {
			maxConcur = concurrent
		}
		mu.Unlock()

		// Sleep long enough for goroutines to genuinely overlap
		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		concurrent--
		mu.Unlock()

		return "done", "", nil
	}

	_ = runner.Run(repos, op)

	// We allow a small margin for timing issues
	if maxConcur > 12 {
		t.Errorf("expected max concurrency <= 10, got %d", maxConcur)
	}
}

func TestRunEmptyRepos(t *testing.T) {
	repos := []*db.Repository{}

	opCalled := false
	op := func(repo *db.Repository) (string, string, error) {
		opCalled = true
		return "success", "", nil
	}

	results := runner.Run(repos, op)

	if opCalled {
		t.Error("operation should not be called with empty repos")
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty repos, got %d", len(results))
	}
}

func TestRunResultOrder(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("first"),
		newTestRepo("second"),
		newTestRepo("third"),
	}

	op := func(repo *db.Repository) (string, string, error) {
		return "ok", "", nil
	}

	results := runner.Run(repos, op)

	expectedAliases := []string{"first", "second", "third"}
	for i, alias := range expectedAliases {
		if results[i].Repo.Alias != alias {
			t.Errorf("results[%d].Repo.Alias = %q, want %q", i, results[i].Repo.Alias, alias)
		}
	}
}

func TestRunMixedStatuses(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("success"),
		newTestRepo("skip"),
		newTestRepo("error"),
	}

	op := func(repo *db.Repository) (string, string, error) {
		switch repo.Alias {
		case "success":
			return "done", "", nil
		case "skip":
			return "", "skipped", nil
		case "error":
			return "", "", fmt.Errorf("error")
		}
		return "", "", nil
	}

	results := runner.Run(repos, op)

	// Find each result and verify status
	statusMap := make(map[string]runner.Status)
	for _, r := range results {
		statusMap[r.Repo.Alias] = r.Status
	}

	if statusMap["success"] != runner.StatusSuccess {
		t.Errorf("success repo should have StatusSuccess, got %v", statusMap["success"])
	}
	if statusMap["skip"] != runner.StatusSkipped {
		t.Errorf("skip repo should have StatusSkipped, got %v", statusMap["skip"])
	}
	if statusMap["error"] != runner.StatusError {
		t.Errorf("error repo should have StatusError, got %v", statusMap["error"])
	}
}

func TestRunMessagePreservation(t *testing.T) {
	repos := []*db.Repository{
		newTestRepo("repo1"),
	}

	expectedMsg := "this is a detailed message with important information"
	op := func(repo *db.Repository) (string, string, error) {
		return expectedMsg, "", nil
	}

	results := runner.Run(repos, op)

	if len(results) != 1 {
		t.Fatalf("expected 1 result")
	}

	if results[0].Message != expectedMsg {
		t.Errorf("Message = %q, want %q", results[0].Message, expectedMsg)
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name    string
		results []runner.Result
		want    bool
	}{
		{
			name:    "nil results",
			results: nil,
			want:    false,
		},
		{
			name:    "empty results",
			results: []runner.Result{},
			want:    false,
		},
		{
			name: "all success",
			results: []runner.Result{
				{Status: runner.StatusSuccess},
				{Status: runner.StatusSuccess},
			},
			want: false,
		},
		{
			name: "one error",
			results: []runner.Result{
				{Status: runner.StatusSuccess},
				{Status: runner.StatusError, Err: fmt.Errorf("boom")},
			},
			want: true,
		},
		{
			name: "skipped only",
			results: []runner.Result{
				{Status: runner.StatusSkipped},
			},
			want: false,
		},
		{
			name: "mixed with error",
			results: []runner.Result{
				{Status: runner.StatusSuccess},
				{Status: runner.StatusSkipped},
				{Status: runner.StatusError, Err: fmt.Errorf("fail")},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.HasErrors(tt.results)
			if got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorCount(t *testing.T) {
	tests := []struct {
		name    string
		results []runner.Result
		want    int
	}{
		{
			name:    "no results",
			results: nil,
			want:    0,
		},
		{
			name: "no errors",
			results: []runner.Result{
				{Status: runner.StatusSuccess},
				{Status: runner.StatusSkipped},
			},
			want: 0,
		},
		{
			name: "one error",
			results: []runner.Result{
				{Status: runner.StatusSuccess},
				{Status: runner.StatusError},
			},
			want: 1,
		},
		{
			name: "all errors",
			results: []runner.Result{
				{Status: runner.StatusError},
				{Status: runner.StatusError},
				{Status: runner.StatusError},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.ErrorCount(tt.results)
			if got != tt.want {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBranchesMissingConfiguredUpstreamFailsAfterMixedBatch(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "healthy")

	brokenDir, _, branch := initRepoWithRemote(t)
	mustRunGit(t, brokenDir, "update-ref", "-d", "refs/remotes/origin/"+branch)
	if _, err := d.AddRepository("broken", "broken", brokenDir, branch); err != nil {
		t.Fatalf("AddRepository broken: %v", err)
	}

	var runErr error
	out := captureOutput(t, func() {
		runErr = runBranches("", false, nil, "")
	})

	if runErr == nil {
		t.Fatal("runBranches() error = nil, want missing configured upstream error")
	}
	if !strings.Contains(runErr.Error(), "1 repository") {
		t.Fatalf("runBranches() error = %q, want failed repository count", runErr)
	}
	if !strings.Contains(out, "healthy") || !strings.Contains(out, "broken") {
		t.Fatalf("mixed batch output omitted a repository:\n%s", out)
	}
	if !strings.Contains(out, "ERROR:") || !strings.Contains(out, "ahead/behind") {
		t.Fatalf("missing configured upstream was not reported as an inspection error:\n%s", out)
	}
}

func TestRunBranchesFetchFailureFailsAfterMixedBatch(t *testing.T) {
	d := setupTestDB(t)
	healthyDir, _, branch := initRepoWithRemote(t)
	if _, err := d.AddRepository("healthy", "healthy", healthyDir, branch); err != nil {
		t.Fatalf("AddRepository healthy: %v", err)
	}

	brokenDir := initRepo(t)
	mustRunGit(t, brokenDir, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	if _, err := d.AddRepository("broken", "broken", brokenDir, branch); err != nil {
		t.Fatalf("AddRepository broken: %v", err)
	}

	var runErr error
	out := captureOutput(t, func() {
		runErr = runBranches("", true, nil, "")
	})

	if runErr == nil {
		t.Fatal("runBranches() error = nil, want requested fetch error")
	}
	if !strings.Contains(runErr.Error(), "1 repository") {
		t.Fatalf("runBranches() error = %q, want failed repository count", runErr)
	}
	if !strings.Contains(out, "healthy") || !strings.Contains(out, "broken") {
		t.Fatalf("mixed batch output omitted a repository:\n%s", out)
	}
	if !strings.Contains(out, "ERROR:") || !strings.Contains(out, "fetch") {
		t.Fatalf("requested fetch failure was not visible:\n%s", out)
	}
}

func TestRunBranchesLocalOnlyShowsUnknownCounts(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "local-only")

	var runErr error
	out := captureOutput(t, func() {
		runErr = runBranches("", false, nil, "")
	})

	if runErr != nil {
		t.Fatalf("runBranches(): %v", runErr)
	}
	if !strings.Contains(out, "none") || !strings.Contains(out, "—") {
		t.Fatalf("local-only branch did not show no upstream and unknown counts:\n%s", out)
	}
	if strings.Contains(out, "up to date") {
		t.Fatalf("local-only branch was misleadingly shown as up to date:\n%s", out)
	}
}

func TestRunBranchesDetachedHeadRemainsNormal(t *testing.T) {
	d := setupTestDB(t)
	repo, dir := newRepo(t, d, "detached")
	mustRunGit(t, dir, "checkout", "--detach", "HEAD")

	var runErr error
	out := captureOutput(t, func() {
		runErr = runBranches("", false, []string{repo.Alias}, "")
	})

	if runErr != nil {
		t.Fatalf("runBranches() detached HEAD: %v", runErr)
	}
	if !strings.Contains(out, "detached") || !strings.Contains(out, "HEAD") {
		t.Fatalf("detached repository did not render a normal row:\n%s", out)
	}
	if strings.Contains(out, "ERROR:") {
		t.Fatalf("detached repository rendered as an error:\n%s", out)
	}
}

package cli

import (
	"strings"
	"testing"
)

func TestBranchesCmdBasics(t *testing.T) {
	cmd := branchesCmd()
	if cmd == nil {
		t.Fatal("branchesCmd() returned nil")
	}
	if !strings.HasPrefix(cmd.Use, "branches") {
		t.Errorf("branchesCmd Use = %q, want prefix %q", cmd.Use, "branches")
	}
	if cmd.Short == "" {
		t.Error("branchesCmd has empty Short description")
	}
	if cmd.RunE == nil {
		t.Error("branchesCmd has no RunE function")
	}
}

func TestBranchesCmdArgsValidation(t *testing.T) {
	cmd := branchesCmd()
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("0 args should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"feature/x"}); err != nil {
		t.Errorf("1 arg should be valid: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("2 args should be rejected by MaximumNArgs(1)")
	}
}

func TestRunBranchesEmptyState(t *testing.T) {
	setupTestDB(t)

	out := captureOutput(t, func() {
		if err := runBranches("", false, nil, ""); err != nil {
			t.Fatalf("runBranches: %v", err)
		}
	})

	if !strings.Contains(out, "No repositories registered") {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}

func TestRunBranchesNoArgMode(t *testing.T) {
	d := setupTestDB(t)

	// A repo sitting on its default branch.
	newRepo(t, d, "main-repo")

	// A repo on a feature branch with a commit not present on the default branch.
	_, featDir := newRepo(t, d, "feat-repo")
	mustRunGit(t, featDir, "checkout", "-q", "-b", "feature/x")
	writeFile(t, featDir, "n.txt", "n\n")
	mustRunGit(t, featDir, "add", ".")
	mustRunGit(t, featDir, "commit", "-q", "-m", "feature commit")

	out := captureOutput(t, func() {
		if err := runBranches("", false, nil, ""); err != nil {
			t.Fatalf("runBranches: %v", err)
		}
	})

	if !strings.Contains(out, "(default)") {
		t.Errorf("expected main-repo to show (default), got:\n%s", out)
	}
	if !strings.Contains(out, "feature/x") {
		t.Errorf("expected feat-repo current branch feature/x, got:\n%s", out)
	}
	if !strings.Contains(out, "not merged") {
		t.Errorf("expected feat-repo to show not merged into default, got:\n%s", out)
	}
}

func TestRunBranchesNoArgAheadCount(t *testing.T) {
	d := setupTestDB(t)

	repoDir, _, branch := initRepoWithRemote(t)
	if _, err := d.AddRepository("svc", "svc", repoDir, branch); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// One local commit not yet pushed — one ahead of origin.
	writeFile(t, repoDir, "a.txt", "a\n")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-q", "-m", "local commit")

	out := captureOutput(t, func() {
		if err := runBranches("", false, nil, ""); err != nil {
			t.Fatalf("runBranches: %v", err)
		}
	})

	if !strings.Contains(out, "1 ahead") {
		t.Errorf("expected svc to show 1 ahead, got:\n%s", out)
	}
}

func TestRunBranchesTargetStates(t *testing.T) {
	d := setupTestDB(t)
	origin := initBareRepo(t)

	// Seed origin with main + feature/JIRA-123 via a throwaway clone.
	seed := cloneRepo(t, origin)
	mustRunGit(t, seed, "config", "user.email", "test@example.com")
	mustRunGit(t, seed, "config", "user.name", "Test User")
	mustRunGit(t, seed, "config", "commit.gpgsign", "false")
	writeFile(t, seed, "base.txt", "base\n")
	mustRunGit(t, seed, "add", ".")
	mustRunGit(t, seed, "commit", "-q", "-m", "init")
	mustRunGit(t, seed, "branch", "-M", "main")
	mustRunGit(t, seed, "push", "-q", "-u", "origin", "main")
	mustRunGit(t, seed, "checkout", "-q", "-b", "feature/JIRA-123")
	writeFile(t, seed, "x.txt", "x\n")
	mustRunGit(t, seed, "add", ".")
	mustRunGit(t, seed, "commit", "-q", "-m", "feature commit")
	mustRunGit(t, seed, "push", "-q", "-u", "origin", "feature/JIRA-123")

	// repoA: has the feature branch locally + remote, and is checked out on it.
	repoA := cloneRepo(t, origin)
	mustRunGit(t, repoA, "config", "user.email", "test@example.com")
	mustRunGit(t, repoA, "config", "user.name", "Test User")
	mustRunGit(t, repoA, "checkout", "-q", "feature/JIRA-123")
	if _, err := d.AddRepository("repoA", "repoA", repoA, "main"); err != nil {
		t.Fatalf("AddRepository repoA: %v", err)
	}

	// repoB: has the feature branch only on origin (remote-tracking), stays on main.
	repoB := cloneRepo(t, origin)
	mustRunGit(t, repoB, "config", "user.email", "test@example.com")
	mustRunGit(t, repoB, "config", "user.name", "Test User")
	if _, err := d.AddRepository("repoB", "repoB", repoB, "main"); err != nil {
		t.Fatalf("AddRepository repoB: %v", err)
	}

	// repoC: an independent repo that never saw the feature branch.
	newRepo(t, d, "repoC")

	out := captureOutput(t, func() {
		if err := runBranches("feature/JIRA-123", false, nil, ""); err != nil {
			t.Fatalf("runBranches: %v", err)
		}
	})

	if !strings.Contains(out, "local+remote") {
		t.Errorf("expected repoA target state local+remote, got:\n%s", out)
	}
	if !strings.Contains(out, "●") {
		t.Errorf("expected on-target marker ● for repoA, got:\n%s", out)
	}
	if !strings.Contains(out, "remote only") {
		t.Errorf("expected repoB target state remote only, got:\n%s", out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("expected repoC target state missing, got:\n%s", out)
	}
}

func TestRunBranchesRepoScoping(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "alpha")
	newRepo(t, d, "beta")

	out := captureOutput(t, func() {
		if err := runBranches("", false, []string{"alpha"}, ""); err != nil {
			t.Fatalf("runBranches: %v", err)
		}
	})

	if !strings.Contains(out, "alpha") {
		t.Errorf("expected alpha in scoped output, got:\n%s", out)
	}
	if strings.Contains(out, "beta") {
		t.Errorf("did not expect beta in --repo alpha output, got:\n%s", out)
	}
}

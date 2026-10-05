package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/git"
)

func TestPruneCmdBasics(t *testing.T) {
	cmd := pruneCmd()
	if cmd.Use != "prune" || cmd.Short == "" || cmd.Long == "" || cmd.RunE == nil {
		t.Fatalf("pruneCmd is incomplete: Use=%q Short=%q", cmd.Use, cmd.Short)
	}
	if err := cmd.Args(cmd, []string{"x"}); err == nil {
		t.Error("prune should reject positional args")
	}
	if err := cmd.ParseFlags([]string{"--yes", "--force", "--no-fetch", "-r", "a,b", "-g", "backend"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	for _, name := range []string{"yes", "force", "no-fetch"} {
		if v, _ := cmd.Flags().GetBool(name); !v {
			t.Errorf("--%s not parsed", name)
		}
	}
	if repos, _ := cmd.Flags().GetStringSlice("repo"); strings.Join(repos, ",") != "a,b" {
		t.Errorf("--repo = %q, want a,b", repos)
	}
}

// newPruneRepo registers a repo with a bare origin holding one branch per
// prune status. The repo is left checked out on feature/current.
func newPruneRepo(t *testing.T) (dir, origin string) {
	t.Helper()
	dir, origin, _ = initRepoWithRemote(t)
	if _, err := database.AddRepository("app", "app", dir, "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	commitOn := func(branch, file string) {
		mustRunGit(t, dir, "checkout", "-q", "-b", branch, "main")
		writeFile(t, dir, file, file+"\n")
		mustRunGit(t, dir, "add", file)
		mustRunGit(t, dir, "commit", "-q", "-m", "work on "+branch)
		mustRunGit(t, dir, "push", "-q", "--set-upstream", "origin", branch)
	}

	commitOn("feature/merged", "merged.txt")
	mustRunGit(t, dir, "checkout", "-q", "main")
	mustRunGit(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature/merged", "feature/merged")
	mustRunGit(t, dir, "push", "-q", "origin", "main")

	// Squash merge: the branch commits never reach main, and GitHub deletes
	// the remote branch.
	commitOn("feature/squashed", "squashed.txt")
	mustRunGit(t, origin, "branch", "-D", "feature/squashed")

	commitOn("feature/ahead", "ahead.txt")
	writeFile(t, dir, "ahead2.txt", "unpushed\n")
	mustRunGit(t, dir, "add", "ahead2.txt")
	mustRunGit(t, dir, "commit", "-q", "-m", "unpushed work")

	commitOn("feature/open", "open.txt")

	mustRunGit(t, dir, "branch", "local-only", "main")

	mustRunGit(t, dir, "checkout", "-q", "-b", "feature/current", "main")
	mustRunGit(t, dir, "push", "-q", "--set-upstream", "origin", "feature/current")
	return dir, origin
}

func localBranchNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	branches, err := git.LocalBranches(dir)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	names := make(map[string]bool, len(branches))
	for _, b := range branches {
		names[b.Name] = true
	}
	return names
}

func assertBranches(t *testing.T, dir string, kept, deleted []string) {
	t.Helper()
	names := localBranchNames(t, dir)
	for _, b := range kept {
		if !names[b] {
			t.Errorf("branch %s was deleted, want kept", b)
		}
	}
	for _, b := range deleted {
		if names[b] {
			t.Errorf("branch %s was kept, want deleted", b)
		}
	}
}

var pruneAlwaysKept = []string{"main", "feature/current", "local-only", "feature/open"}

func TestRunPruneDryRunDeletesNothing(t *testing.T) {
	setupTestDB(t)
	dir, _ := newPruneRepo(t)

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{}); err != nil {
		t.Fatalf("runPrune: %v", err)
	}

	assertBranches(t, dir, append(pruneAlwaysKept, "feature/merged", "feature/squashed", "feature/ahead"), nil)
	for _, want := range []string{
		"DRY RUN",
		"merged", "upstream gone", "1 ahead", "not pushed", "current", "default", "not merged",
		"work on feature/squashed",
		"2 branch(es) would be deleted in 1 repository(ies)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunPruneYesDeletesMergedAndGone(t *testing.T) {
	setupTestDB(t)
	dir, _ := newPruneRepo(t)

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{yes: true}); err != nil {
		t.Fatalf("runPrune: %v\n%s", err, out.String())
	}

	assertBranches(t, dir, append(pruneAlwaysKept, "feature/ahead"), []string{"feature/merged", "feature/squashed"})
	if strings.Contains(out.String(), "DRY RUN") {
		t.Errorf("--yes output should not say DRY RUN:\n%s", out.String())
	}
}

func TestRunPruneForceDeletesAhead(t *testing.T) {
	setupTestDB(t)
	dir, _ := newPruneRepo(t)

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{yes: true, force: true}); err != nil {
		t.Fatalf("runPrune: %v\n%s", err, out.String())
	}

	assertBranches(t, dir, pruneAlwaysKept, []string{"feature/merged", "feature/squashed", "feature/ahead"})
}

func TestRunPruneNoFetchDoesNotSeeRemoteDeletion(t *testing.T) {
	setupTestDB(t)
	dir, _ := newPruneRepo(t)

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{yes: true, noFetch: true}); err != nil {
		t.Fatalf("runPrune: %v\n%s", err, out.String())
	}

	assertBranches(t, dir, append(pruneAlwaysKept, "feature/squashed"), []string{"feature/merged"})
}

func TestRunPruneUsesRemoteDefaultBranch(t *testing.T) {
	setupTestDB(t)
	dir, _ := newPruneRepo(t)

	// Local main falls behind origin/main; the branch merged on the remote
	// must still count as merged.
	mustRunGit(t, dir, "checkout", "-q", "main")
	mustRunGit(t, dir, "reset", "-q", "--hard", "HEAD~1")
	mustRunGit(t, dir, "checkout", "-q", "feature/current")

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{yes: true, noFetch: true}); err != nil {
		t.Fatalf("runPrune: %v\n%s", err, out.String())
	}
	assertBranches(t, dir, nil, []string{"feature/merged"})
}

func TestRunPruneMissingDefaultBranchIsRepoError(t *testing.T) {
	d := setupTestDB(t)
	dir := initRepo(t)
	if _, err := d.AddRepository("app", "app", dir, "develop"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	var out bytes.Buffer
	err := runPrune(&out, pruneOptions{noFetch: true})
	if err == nil {
		t.Fatal("runPrune should fail when the default branch is missing")
	}
	if !strings.Contains(out.String(), `default branch "develop" not found`) {
		t.Errorf("output missing the repo error:\n%s", out.String())
	}
}

func TestRunPruneEmptyState(t *testing.T) {
	setupTestDB(t)

	var out bytes.Buffer
	if err := runPrune(&out, pruneOptions{}); err != nil {
		t.Fatalf("runPrune: %v", err)
	}
	if !strings.Contains(out.String(), "No repositories registered") {
		t.Errorf("expected empty-state message, got:\n%s", out.String())
	}
}

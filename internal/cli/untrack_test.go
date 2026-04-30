package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

func TestUntrackCmdStructure(t *testing.T) {
	cmd := untrackCmd()
	if cmd.Use != "untrack" {
		t.Errorf("Use = %q, want %q", cmd.Use, "untrack")
	}
	if cmd.Short == "" {
		t.Error("Short is empty")
	}

	f := cmd.Flags().Lookup("repo")
	if f == nil {
		t.Fatal("--repo flag not registered")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

func TestUntrackSubcommandRegistered(t *testing.T) {
	root := Root("test")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "untrack" {
			found = true
			break
		}
	}
	if !found {
		t.Error("untrack subcommand not registered")
	}
}

func TestRunUntrackNoRepos(t *testing.T) {
	d := setupTestDB(t)
	_ = d

	err := runUntrackWithUI(fakeUI{}, nil)
	if err == nil {
		t.Fatal("expected error for no repos")
	}
}

func TestRunUntrackCanceledSelection(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "test-repo")

	err := runUntrackWithUI(fakeUI{
		selectErr: errors.New("canceled"),
	}, nil)
	if err != nil {
		t.Fatalf("canceled selection should not be fatal, got: %v", err)
	}
}

func TestRunUntrackWithFiles(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "test-repo")

	writeFile(t, dir, "secret.env", "SECRET=abc\n")
	mustRunGit(t, dir, "add", "secret.env")
	mustRunGit(t, dir, "commit", "-m", "add secret")

	err := runUntrackWithUI(fakeUI{
		selectRepos: []*db.Repository{{Alias: "test-repo", Path: dir}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := mustRunGit(t, dir, "status", "--porcelain")
	if !strings.Contains(out, "secret.env") {
		t.Error("expected secret.env to appear in status after untrack")
	}
}

func TestRunUntrackWithRepoFlag(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "test-repo")

	writeFile(t, dir, "build.log", "log output\n")
	mustRunGit(t, dir, "add", "build.log")
	mustRunGit(t, dir, "commit", "-m", "add log")

	err := runUntrackWithUI(fakeUI{
		selectRepos: []*db.Repository{{Alias: "test-repo", Path: dir}},
	}, []string{"test-repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

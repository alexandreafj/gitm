package cli

import (
	"errors"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestTrackCmdStructure(t *testing.T) {
	cmd := trackCmd()
	if cmd.Use != "track" {
		t.Errorf("Use = %q, want %q", cmd.Use, "track")
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

func TestTrackSubcommandRegistered(t *testing.T) {
	root := Root("test")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "track" {
			found = true
			break
		}
	}
	if !found {
		t.Error("track subcommand not registered")
	}
}

func TestRunTrackNoRepos(t *testing.T) {
	d := setupTestDB(t)
	_ = d

	err := runTrackWithUI(fakeUI{}, nil)
	if err == nil {
		t.Fatal("expected error for no repos")
	}
}

func TestRunTrackNoUntrackedFiles(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "test-repo")

	err := runTrackWithUI(fakeUI{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTrackWithUntrackedFiles(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "test-repo")

	writeFile(t, dir, "newfile.txt", "untracked content\n")

	err := runTrackWithUI(fakeUI{
		selectRepos: []*db.Repository{{Alias: "test-repo", Path: dir}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := mustRunGit(t, dir, "status", "--porcelain")
	if out == "" {
		t.Error("expected staged file after track")
	}
}

func TestRunTrackCanceledSelection(t *testing.T) {
	d := setupTestDB(t)
	newRepo(t, d, "test-repo")

	err := runTrackWithUI(fakeUI{
		selectErr: errors.New("canceled"),
	}, nil)
	if err != nil {
		t.Fatalf("canceled selection should not be fatal, got: %v", err)
	}
}

func TestRunTrackWithRepoFlag(t *testing.T) {
	d := setupTestDB(t)
	repo, dir := newRepo(t, d, "test-repo")
	_ = repo

	writeFile(t, dir, "flagfile.txt", "content\n")

	err := runTrackWithUI(fakeUI{}, []string{"test-repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

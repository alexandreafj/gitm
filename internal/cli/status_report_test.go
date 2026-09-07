package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

func TestRepoStatusNeedsAttention(t *testing.T) {
	zero, one := 0, 1
	clean, dirty := false, true
	for _, tt := range []struct {
		name   string
		status repoStatus
		want   bool
	}{
		{"healthy", repoStatus{Upstream: "origin/main", Dirty: &clean, Ahead: &zero, Behind: &zero}, false},
		{"dirty", repoStatus{Upstream: "origin/main", Dirty: &dirty}, true},
		{"ahead", repoStatus{Upstream: "origin/main", Ahead: &one}, true},
		{"behind", repoStatus{Upstream: "origin/main", Behind: &one}, true},
		{"error", repoStatus{Upstream: "origin/main", Error: "fetch failed"}, true},
		{"no upstream", repoStatus{}, true},
		{"detached", repoStatus{Upstream: "origin/main", Detached: true}, true},
		{"unborn", repoStatus{Upstream: "origin/main", Unborn: true}, true},
		{"conflicts", repoStatus{Upstream: "origin/main", Conflicts: true}, true},
		{"operation", repoStatus{Upstream: "origin/main", Operations: []string{"bisect"}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.needsAttention(); got != tt.want {
				t.Fatalf("needsAttention=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectRepoStatusSpecialStates(t *testing.T) {
	database = setupTestDB(t)
	t.Run("unusable tracking configuration", func(t *testing.T) {
		repo, dir := newRepo(t, database, "unusable")
		mustRunGit(t, dir, "config", "branch.main.remote", "origin")
		mustRunGit(t, dir, "config", "branch.main.merge", "refs/heads/main")
		s := collectRepoStatus(repo, false)
		if s.Error == "" || s.RemoteState != "unknown" || s.Ahead != nil || s.Dirty == nil {
			t.Fatalf("unusable upstream must preserve local state with unknown counts: %+v", s)
		}
		output, err := renderStatusTable(statusReport{Repositories: []repoStatus{s}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output, "no upstream") || !strings.Contains(output, "unknown") {
			t.Fatalf("configured upstream must not be labeled missing: %s", output)
		}
	})
	t.Run("unborn", func(t *testing.T) {
		dir := t.TempDir()
		mustRunGit(t, dir, "init", "-b", "new")
		s := collectRepoStatus(&db.Repository{Alias: "new", Path: dir}, false)
		if !s.Unborn || s.Branch != "new" || s.Error != "" || s.Dirty == nil || *s.Dirty || s.Ahead != nil {
			t.Fatalf("status=%+v", s)
		}
	})
	t.Run("detached", func(t *testing.T) {
		repo, dir := newRepo(t, database, "detached")
		mustRunGit(t, dir, "checkout", "--detach")
		s := collectRepoStatus(repo, false)
		if !s.Detached || s.Error != "" || s.Ahead != nil {
			t.Fatalf("status=%+v", s)
		}
	})
	t.Run("merge conflict", func(t *testing.T) {
		repo, dir := newRepo(t, database, "conflict")
		mustRunGit(t, dir, "checkout", "-b", "other")
		writeFile(t, dir, "README.md", "other\n")
		mustRunGit(t, dir, "commit", "-am", "other")
		mustRunGit(t, dir, "checkout", "main")
		writeFile(t, dir, "README.md", "main\n")
		mustRunGit(t, dir, "commit", "-am", "main")
		if _, err := git.Merge(dir, "other"); err == nil {
			t.Fatal("expected conflict")
		}
		s := collectRepoStatus(repo, false)
		if !s.Conflicts || s.Error != "" || len(s.Operations) != 1 || s.Operations[0] != "merge" {
			t.Fatalf("status=%+v", s)
		}
	})
	t.Run("missing tracking ref", func(t *testing.T) {
		repo, dir := newRepo(t, database, "gone")
		mustRunGit(t, dir, "remote", "add", "origin", dir)
		mustRunGit(t, dir, "fetch", "origin")
		mustRunGit(t, dir, "branch", "--set-upstream-to=origin/main", "main")
		mustRunGit(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
		s := collectRepoStatus(repo, false)
		if s.Error == "" || s.Upstream != "origin/main" || s.Ahead != nil || s.RemoteState != "unknown" {
			t.Fatalf("status=%+v", s)
		}
	})
}

func TestWriteStatusReportOutputFailures(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		file, err := os.Create(filepath.Join(t.TempDir(), "closed"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if err := writeStatusReport(file, "default", nil, statusOptions{json: jsonOutput}); err == nil {
			t.Fatal("expected output failure")
		}
	}
}

func TestStatusAttentionEmptyAndContext(t *testing.T) {
	database = setupTestDB(t)
	if _, err := database.CreateContext("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SetActiveContext("work"); err != nil {
		t.Fatal(err)
	}
	_, dir := newRepo(t, database, "healthy")
	mustRunGit(t, dir, "remote", "add", "origin", dir)
	mustRunGit(t, dir, "fetch", "origin")
	mustRunGit(t, dir, "branch", "--set-upstream-to=origin/main", "main")
	var out bytes.Buffer
	if err := runStatusWithOptions(&out, nil, "", statusOptions{attention: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Context: work") || !strings.Contains(out.String(), "No repositories need attention") || strings.Contains(out.String(), "healthy") {
		t.Fatalf("output=%s", out.String())
	}
}

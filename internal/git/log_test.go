package git_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandreafj/gitm/internal/git"
)

// commitAt commits file with both author and committer dates set to age ago.
// Extra args go before "commit", e.g. "-c", "user.email=other@example.com".
func commitAt(t *testing.T, dir, file, msg string, age time.Duration, extra ...string) {
	t.Helper()
	writeFile(t, dir, file, msg+"\n")
	mustRunGit(t, dir, "add", file)
	when := fmt.Sprintf("@%d +0000", time.Now().Add(-age).Unix())
	cmd := exec.Command("git", append(extra, "commit", "-q", "-m", msg)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit %q: %v\n%s", msg, err, out)
	}
}

func subjects(entries []git.LogEntry) []string {
	s := make([]string, len(entries))
	for i, e := range entries {
		s[i] = e.Subject
	}
	return s
}

func assertSubjects(t *testing.T, got []git.LogEntry, want ...string) {
	t.Helper()
	gs := subjects(got)
	if fmt.Sprint(gs) != fmt.Sprint(want) {
		t.Errorf("subjects = %q, want %q", gs, want)
	}
}

func TestConfigValue(t *testing.T) {
	dir := initRepo(t)

	got, err := git.ConfigValue(dir, "user.email")
	if err != nil || got != "test@example.com" {
		t.Errorf("ConfigValue(user.email) = %q, %v; want test@example.com", got, err)
	}

	got, err = git.ConfigValue(dir, "gitm.unset-key")
	if err != nil || got != "" {
		t.Errorf("ConfigValue(unset) = %q, %v; want empty, nil", got, err)
	}

	if _, err := git.ConfigValue(filepath.Join(dir, "missing"), "user.email"); err == nil {
		t.Error("ConfigValue on a missing directory should fail")
	}
}

func TestLogParsesEntries(t *testing.T) {
	dir := initRepo(t)
	makeCommit(t, dir, "a.txt", "a\n", "subject with\ttab and spaces")

	entries, err := git.Log(dir, git.LogOptions{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	assertSubjects(t, entries, "subject with\ttab and spaces", "initial commit")
	head := mustRunGit(t, dir, "rev-parse", "--short", "HEAD")
	if entries[0].Hash != head {
		t.Errorf("hash = %q, want %q", entries[0].Hash, head)
	}
	if entries[0].Date == "" {
		t.Error("relative date is empty")
	}
}

func TestLogFilters(t *testing.T) {
	dir := initRepo(t)
	other := []string{"-c", "user.email=other@example.com", "-c", "user.name=Other"}
	commitAt(t, dir, "old.txt", "ten days old", 10*24*time.Hour)
	commitAt(t, dir, "mid.txt", "three days old", 3*24*time.Hour)
	commitAt(t, dir, "theirs.txt", "by other", time.Hour, other...)
	commitAt(t, dir, "mine.txt", "by me", time.Hour)
	mustRunGit(t, dir, "checkout", "-q", "-b", "side")
	commitAt(t, dir, "side.txt", "on side", time.Minute)
	mustRunGit(t, dir, "checkout", "-q", "main")

	tests := []struct {
		name string
		opts git.LogOptions
		want []string
	}{
		{"since", git.LogOptions{Since: "5 days ago"}, []string{"by me", "by other", "three days old"}},
		{"since and until", git.LogOptions{Since: "15 days ago", Until: "5 days ago"}, []string{"ten days old"}},
		{"author", git.LogOptions{Since: "2 days ago", Author: "<other@example.com>"}, []string{"by other"}},
		{"all branches", git.LogOptions{Since: "2 days ago", AllBranches: true}, []string{"on side", "by me", "by other"}},
		{"max count", git.LogOptions{Since: "5 days ago", MaxCount: 2}, []string{"by me", "by other"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := git.Log(dir, tt.opts)
			if err != nil {
				t.Fatalf("Log: %v", err)
			}
			assertSubjects(t, got, tt.want...)
		})
	}
}

func TestLogNoMatchesIsEmpty(t *testing.T) {
	dir := initRepo(t)
	got, err := git.Log(dir, git.LogOptions{Author: "<nobody@example.com>"})
	if err != nil || len(got) != 0 {
		t.Errorf("Log = %v, %v; want empty, nil", got, err)
	}
}

func TestLogMissingDirectoryFails(t *testing.T) {
	if _, err := git.Log(filepath.Join(t.TempDir(), "missing"), git.LogOptions{}); err == nil {
		t.Error("Log on a missing directory should fail")
	}
}

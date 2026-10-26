package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestLogCmdBasics(t *testing.T) {
	cmd := logCmd()
	if cmd.Use != "log" || cmd.Short == "" || cmd.Long == "" || cmd.RunE == nil {
		t.Fatalf("logCmd is incomplete: Use=%q Short=%q", cmd.Use, cmd.Short)
	}
	if err := cmd.Args(cmd, []string{"x"}); err == nil {
		t.Error("log should reject positional args")
	}
	if got := cmd.Flags().Lookup("since").DefValue; got != "yesterday" {
		t.Errorf("--since default = %q, want yesterday", got)
	}
	if got := cmd.Flags().Lookup("max-count").DefValue; got != "0" {
		t.Errorf("--max-count default = %q, want 0", got)
	}

	args := []string{"--since", "last friday", "--until", "today", "--author", "jane", "--current-branch", "-n", "3", "-r", "a,b", "-g", "backend"}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	for name, want := range map[string]string{"since": "last friday", "until": "today", "author": "jane", "group": "backend"} {
		if got, err := cmd.Flags().GetString(name); err != nil || got != want {
			t.Errorf("--%s = %q, %v; want %q", name, got, err, want)
		}
	}
	if v, err := cmd.Flags().GetBool("current-branch"); err != nil || !v {
		t.Errorf("--current-branch = %v, %v; want true", v, err)
	}
	if n, err := cmd.Flags().GetInt("max-count"); err != nil || n != 3 {
		t.Errorf("-n = %d, %v; want 3", n, err)
	}
	if repos, err := cmd.Flags().GetStringSlice("repo"); err != nil || strings.Join(repos, ",") != "a,b" {
		t.Errorf("--repo = %q, %v; want a,b", repos, err)
	}
}

func TestLogCmdAuthorAndAllAuthorsAreExclusive(t *testing.T) {
	cmd := logCmd()
	if err := cmd.ParseFlags([]string{"--author", "jane", "--all-authors"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if err := cmd.ValidateFlagGroups(); err == nil {
		t.Error("--author with --all-authors should be rejected")
	}
}

func TestRunLogRejectsNegativeMaxCount(t *testing.T) {
	if err := runLog(&bytes.Buffer{}, logOptions{maxCount: -1}); err == nil {
		t.Error("runLog should reject --max-count -1")
	}
}

// commitAt commits file in dir with author and committer dates age ago.
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

// otherAuthor contains the test user's email as a substring, so it also
// checks that the default filter matches the whole address.
var otherAuthor = []string{"-c", "user.email=not-test@example.com", "-c", "user.name=Other"}

func assertLogOutput(t *testing.T, out string, present, absent []string) {
	t.Helper()
	for _, s := range present {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q:\n%s", s, out)
		}
	}
	for _, s := range absent {
		if strings.Contains(out, s) {
			t.Errorf("output should not contain %q:\n%s", s, out)
		}
	}
}

func outputLines(out string) map[string]bool {
	lines := make(map[string]bool)
	for _, l := range strings.Split(out, "\n") {
		lines[l] = true
	}
	return lines
}

func TestRunLogAuthorFilter(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "app")
	commitAt(t, dir, "mine.txt", "my work", time.Hour)
	commitAt(t, dir, "theirs.txt", "their work", time.Hour, otherAuthor...)

	tests := []struct {
		name            string
		opts            logOptions
		present, absent []string
	}{
		{"default is current user", logOptions{since: "yesterday"},
			[]string{"app", "my work", "initial commit", "2 commit(s) across 1 repository(ies)."}, []string{"their work"}},
		{"all authors", logOptions{since: "yesterday", allAuthors: true},
			[]string{"my work", "their work", "3 commit(s)"}, nil},
		{"explicit author", logOptions{since: "yesterday", author: "Other"},
			[]string{"their work", "1 commit(s)"}, []string{"my work"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := runLog(&out, tt.opts); err != nil {
				t.Fatalf("runLog: %v\n%s", err, out.String())
			}
			assertLogOutput(t, out.String(), tt.present, tt.absent)
		})
	}
}

func TestRunLogSinceUntil(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "app")
	commitAt(t, dir, "old.txt", "ten days old", 10*24*time.Hour)
	commitAt(t, dir, "mid.txt", "three days old", 3*24*time.Hour)

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "5 days ago"}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"three days old"}, []string{"ten days old"})

	out.Reset()
	if err := runLog(&out, logOptions{since: "15 days ago", until: "5 days ago"}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"ten days old", "1 commit(s)"}, []string{"three days old", "initial commit"})

	out.Reset()
	if err := runLog(&out, logOptions{since: "yesterday"}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"No commits found."}, []string{"ten days old", "three days old"})
}

func TestRunLogAllBranchesByDefault(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "app")
	mustRunGit(t, dir, "checkout", "-q", "-b", "feature/side")
	commitAt(t, dir, "side.txt", "side branch work", time.Minute)
	mustRunGit(t, dir, "checkout", "-q", "main")

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "yesterday"}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"side branch work", "initial commit"}, nil)

	out.Reset()
	if err := runLog(&out, logOptions{since: "yesterday", currentBranch: true}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"initial commit"}, []string{"side branch work"})
}

func TestRunLogMaxCount(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "app")
	commitAt(t, dir, "1.txt", "first", 3*time.Hour)
	commitAt(t, dir, "2.txt", "second", 2*time.Hour)
	commitAt(t, dir, "3.txt", "third", time.Hour)

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "yesterday", maxCount: 2}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	assertLogOutput(t, out.String(), []string{"third", "second", "2 commit(s)"}, []string{"first"})
	if strings.Index(out.String(), "third") > strings.Index(out.String(), "second") {
		t.Errorf("commits should be newest first:\n%s", out.String())
	}
}

func TestRunLogOmitsQuietReposReportsFailuresInAliasOrder(t *testing.T) {
	d := setupTestDB(t)
	_, zeta := newRepo(t, d, "zeta")
	commitAt(t, zeta, "z.txt", "zeta work", time.Hour)
	_, alpha := newRepo(t, d, "alpha")
	commitAt(t, alpha, "a.txt", "alpha work", time.Hour)
	_, quiet := newRepo(t, d, "quiet")
	mustRunGit(t, quiet, "config", "user.email", "someone-else@example.com")
	_, broken := newRepo(t, d, "broken")
	if err := os.RemoveAll(broken); err != nil {
		t.Fatalf("remove broken repo: %v", err)
	}

	for _, aliases := range [][]string{nil, {"zeta", "quiet", "broken", "alpha"}} {
		var out bytes.Buffer
		err := runLog(&out, logOptions{since: "yesterday", repoAliases: aliases})
		if err == nil || !strings.Contains(err.Error(), "1 repository(ies)") {
			t.Fatalf("runLog error = %v, want failure for 1 repository\n%s", err, out.String())
		}
		s := out.String()
		lines := outputLines(s)
		if lines["quiet"] {
			t.Errorf("repo without matching commits should be omitted:\n%s", s)
		}
		if !lines["broken"] || !strings.Contains(s, "ERROR: ") {
			t.Errorf("broken repo should show an error entry:\n%s", s)
		}
		a, b, z := strings.Index(s, "alpha work"), strings.Index(s, "\nbroken\n"), strings.Index(s, "zeta work")
		if a < 0 || b < 0 || z < 0 || a > b || b > z {
			t.Errorf("repos not in alias order (alpha %d, broken %d, zeta %d):\n%s", a, b, z, s)
		}
		if !strings.Contains(s, "4 commit(s) across 2 repository(ies).") {
			t.Errorf("summary wrong:\n%s", s)
		}
	}
}

func TestRunLogWarnsWhenNoIdentity(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "anon")
	commitAt(t, dir, "theirs.txt", "their work", time.Hour, otherAuthor...)
	mustRunGit(t, dir, "config", "--unset", "user.email")
	mustRunGit(t, dir, "config", "--unset", "user.name")

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "yesterday"}); err != nil {
		t.Fatalf("runLog: %v\n%s", err, out.String())
	}
	s := out.String()
	if strings.Count(s, "Warning:") != 1 || !strings.Contains(s, "Warning: anon: no user.email or user.name configured") {
		t.Errorf("want exactly one warning naming anon:\n%s", s)
	}
	assertLogOutput(t, s, []string{"their work", "initial commit"}, nil)
}

func TestRunLogFallsBackToUserName(t *testing.T) {
	d := setupTestDB(t)
	_, dir := newRepo(t, d, "app")
	commitAt(t, dir, "theirs.txt", "their work", time.Hour, otherAuthor...)
	mustRunGit(t, dir, "config", "--unset", "user.email")

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "yesterday"}); err != nil {
		t.Fatalf("runLog: %v\n%s", err, out.String())
	}
	assertLogOutput(t, out.String(), []string{"initial commit"}, []string{"their work", "Warning:"})
}

func TestRunLogEmptyState(t *testing.T) {
	setupTestDB(t)

	var out bytes.Buffer
	if err := runLog(&out, logOptions{since: "yesterday"}); err != nil {
		t.Fatalf("runLog: %v", err)
	}
	if !strings.Contains(out.String(), "No repositories registered") {
		t.Errorf("expected empty-state message, got:\n%s", out.String())
	}
}

func TestCollectLogMissingDirectory(t *testing.T) {
	repo := &db.Repository{Alias: "gone", Path: t.TempDir() + "/missing"}
	if l := collectLog(repo, logOptions{}); l.err == nil {
		t.Error("collectLog should fail for a missing directory")
	}
	if l := collectLog(repo, logOptions{allAuthors: true}); l.err == nil {
		t.Error("collectLog --all-authors should fail for a missing directory")
	}
}

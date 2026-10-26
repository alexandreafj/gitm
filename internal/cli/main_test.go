package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

const templateBranch = "main"

// Written once by TestMain before any test runs, then only read. Copying a
// prepared repository or database is much cheaper than repeating git init,
// config, the initial commit and push, or every schema migration (each its
// own committed SQLite transaction), in every test.
var templates struct {
	repo       string
	remoteRepo string
	origin     string
	db         []byte
}

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gitm-cli-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	// A throwaway global config isolates every git invocation in this package
	// from the developer's own: a global core.hooksPath (lefthook, commit-msg
	// linters) would otherwise run against the test repos and fail commits.
	// It also turns off auto maintenance and receive-side auto gc, which spawn
	// one more git process per commit, fetch or push although test repos never
	// reach their thresholds. GIT_CONFIG_COUNT cannot carry these settings:
	// git strips it from the receive-pack side of a local push.
	globalConfig := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(globalConfig, []byte("[maintenance]\n\tauto = false\n[receive]\n\tautogc = false\n"), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	os.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	if err := preferGitExecPath(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := buildRepoTemplates(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := buildDBTemplate(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

// On macOS /usr/bin/git is an xcrun shim that adds roughly 10ms to every
// call. The exec-path holds the binary the shim resolves to, so putting it
// first on PATH runs the same git without the shim.
func preferGitExecPath() error {
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return fmt.Errorf("git --exec-path: %w", err)
	}
	execPath := strings.TrimSpace(string(out))
	if _, err := os.Stat(filepath.Join(execPath, "git")); err != nil {
		return nil
	}
	return os.Setenv("PATH", execPath+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func buildRepoTemplates(root string) error {
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		return fmt.Errorf("create template dir: %w", err)
	}
	if err := gitSteps(repo,
		[]string{"init", "-b", templateBranch},
		[]string{"config", "user.email", "test@example.com"},
		[]string{"config", "user.name", "Test User"},
		[]string{"config", "commit.gpgsign", "false"},
	); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# test repo\n"), 0644); err != nil {
		return fmt.Errorf("write template README: %w", err)
	}
	if err := gitSteps(repo, []string{"add", "."}, []string{"commit", "-m", "initial commit"}); err != nil {
		return err
	}

	origin := filepath.Join(root, "origin")
	remoteRepo := filepath.Join(root, "remote-repo")
	if err := os.CopyFS(remoteRepo, os.DirFS(repo)); err != nil {
		return fmt.Errorf("copy template repo: %w", err)
	}
	if err := gitSteps(root, []string{"init", "--bare", "--initial-branch=" + templateBranch, origin}); err != nil {
		return err
	}
	if err := gitSteps(remoteRepo,
		[]string{"remote", "add", "origin", origin},
		[]string{"push", "--set-upstream", "origin", templateBranch},
	); err != nil {
		return err
	}

	// Git never runs the *.sample hooks, yet they are half the files of every copy.
	for _, hooks := range []string{filepath.Join(repo, ".git", "hooks"), filepath.Join(remoteRepo, ".git", "hooks"), filepath.Join(origin, "hooks")} {
		samples, err := filepath.Glob(filepath.Join(hooks, "*.sample"))
		if err != nil {
			return fmt.Errorf("list sample hooks: %w", err)
		}
		for _, sample := range samples {
			if err := os.Remove(sample); err != nil {
				return fmt.Errorf("remove sample hook: %w", err)
			}
		}
	}

	templates.repo = repo
	templates.remoteRepo = remoteRepo
	templates.origin = origin
	return nil
}

func buildDBTemplate(root string) error {
	path := filepath.Join(root, "template.db")
	d, err := db.Open(path)
	if err != nil {
		return fmt.Errorf("open template db: %w", err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("close template db: %w", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read template db: %w", err)
	}
	templates.db = content
	return nil
}

func gitSteps(dir string, steps ...[]string) error {
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v in %s: %w\n%s", args, dir, err, out)
		}
	}
	return nil
}

func TestRepoTemplateCopiesBehaveLikeFreshRepos(t *testing.T) {
	dir := initRepo(t)
	// Plumbing does not refresh the index, so a stale copy would show up here.
	mustRunGit(t, dir, "diff-files", "--quiet")
	mustRunGit(t, dir, "diff-index", "--quiet", "HEAD")
	if got := mustRunGit(t, dir, "config", "user.email"); got != "test@example.com" {
		t.Errorf("user.email = %q, want test@example.com", got)
	}
	if got := mustRunGit(t, dir, "log", "--format=%s"); got != "initial commit" {
		t.Errorf("history = %q, want only the initial commit", got)
	}
	if out := mustRunGit(t, dir, "remote"); out != "" {
		t.Errorf("plain repo has remotes: %q", out)
	}

	repo, origin, branch := initRepoWithRemote(t)
	mustRunGit(t, repo, "diff-files", "--quiet")
	if branch != gitCurrentBranch(t, repo) {
		t.Errorf("branch = %q, want the checked-out branch %q", branch, gitCurrentBranch(t, repo))
	}
	if got := mustRunGit(t, repo, "remote", "get-url", "origin"); got != origin {
		t.Errorf("origin url = %q, want %q", got, origin)
	}
	if got := mustRunGit(t, repo, "rev-parse", "--abbrev-ref", "@{upstream}"); got != "origin/"+branch {
		t.Errorf("upstream = %q, want origin/%s", got, branch)
	}
	if local, remote := mustRunGit(t, repo, "rev-parse", "HEAD"), mustRunGit(t, origin, "rev-parse", branch); local != remote {
		t.Errorf("origin %s = %s, want local HEAD %s", branch, remote, local)
	}
	mustRunGit(t, repo, "fetch", "origin")
}

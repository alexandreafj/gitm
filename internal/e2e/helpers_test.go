package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitmBinary holds the path to the built gitm binary.
// Set once in TestMain.
var gitmBinary string

func TestMain(m *testing.M) {
	// Build the binary once for all e2e tests.
	binary, err := buildGitm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build gitm: %v\n", err)
		os.Exit(1)
	}
	gitmBinary = binary

	os.Exit(m.Run())
}

// buildGitm compiles the gitm binary into a temp directory and returns its path.
func buildGitm() (string, error) {
	dir, err := os.MkdirTemp("", "gitm-e2e-bin-*")
	if err != nil {
		return "", err
	}

	binary := filepath.Join(dir, "gitm")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	// Find the project root (two levels up from internal/e2e)
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	cmd := exec.Command("go", "build",
		"-ldflags", "-X main.version=e2e-test",
		"-o", binary,
		"./cmd/gitm",
	)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build: %w\n%s", err, out)
	}
	return binary, nil
}

// --------------------------------------------------------------------------
// Test environment helpers
// --------------------------------------------------------------------------

// testEnv represents an isolated test environment with its own HOME dir.
type testEnv struct {
	t       *testing.T
	homeDir string
	dataDir string // ~/.gitm/ equivalent
}

// newTestEnv creates an isolated environment for a single test.
// It sets up a fresh HOME directory so gitm gets its own database.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir()
	dataDir := filepath.Join(home, ".gitm")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dataDir, err)
	}
	return &testEnv{
		t:       t,
		homeDir: home,
		dataDir: dataDir,
	}
}

// --------------------------------------------------------------------------
// Running gitm
// --------------------------------------------------------------------------

// result holds the output of a gitm invocation.
type result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runGitm executes the gitm binary with the given arguments in this test environment.
func (e *testEnv) runGitm(args ...string) result {
	e.t.Helper()
	return e.runGitmInDir("", args...)
}

// runGitmInDir executes gitm with a specific working directory.
func (e *testEnv) runGitmInDir(dir string, args ...string) result {
	e.t.Helper()

	cmd := exec.Command(gitmBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+e.homeDir,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			e.t.Fatalf("failed to run gitm %v: %v", args, err)
		}
	}

	return result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// --------------------------------------------------------------------------
// Git repo setup helpers
// --------------------------------------------------------------------------

// initRepo creates a new git repository in a temp directory with an initial commit.
// Returns the repo path.
func (e *testEnv) initRepo(name string) string {
	e.t.Helper()
	dir := filepath.Join(e.t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatalf("MkdirAll: %v", err)
	}
	e.mustGit(dir, "init", "-b", "main")
	e.mustGit(dir, "config", "user.email", "test@e2e.dev")
	e.mustGit(dir, "config", "user.name", "E2E Test")
	e.mustGit(dir, "config", "commit.gpgsign", "false")
	e.writeFile(dir, "README.md", "# "+name+"\n")
	e.mustGit(dir, "add", ".")
	e.mustGit(dir, "commit", "-m", "initial commit")
	return dir
}

// initRepoWithRemote creates a repo with a bare remote "origin" and pushes the initial commit.
// Returns (repoDir, bareOriginDir).
func (e *testEnv) initRepoWithRemote(name string) (string, string) {
	e.t.Helper()
	// Create bare remote
	origin := filepath.Join(e.t.TempDir(), name+"-origin.git")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		e.t.Fatalf("MkdirAll: %v", err)
	}
	e.mustGit(origin, "init", "--bare", "--initial-branch=main")

	// Create working repo
	repo := e.initRepo(name)
	e.mustGit(repo, "remote", "add", "origin", origin)
	e.mustGit(repo, "push", "--set-upstream", "origin", "main")

	return repo, origin
}

// cloneRepo clones from an origin into a new temp directory.
func (e *testEnv) cloneRepo(origin, name string) string {
	e.t.Helper()
	dir := filepath.Join(e.t.TempDir(), name)
	cmd := exec.Command("git", "clone", origin, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git clone: %v\n%s", err, out)
	}
	e.mustGit(dir, "config", "user.email", "test@e2e.dev")
	e.mustGit(dir, "config", "user.name", "E2E Test")
	e.mustGit(dir, "config", "commit.gpgsign", "false")
	return dir
}

// mustGit runs a git command in the given directory, failing the test on error.
func (e *testEnv) mustGit(dir string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimRight(string(out), "\r\n")
}

// writeFile creates a file with the given content.
func (e *testEnv) writeFile(dir, name, content string) {
	e.t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatalf("WriteFile %s: %v", name, err)
	}
}

// fileExists checks if a file exists at the given path.
func (e *testEnv) fileExists(path string) bool {
	e.t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// currentBranch returns the current branch of a git repo.
func (e *testEnv) currentBranch(dir string) string {
	e.t.Helper()
	return e.mustGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// branchExists checks if a branch exists locally in the repo.
func (e *testEnv) branchExists(dir, branch string) bool {
	e.t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// --------------------------------------------------------------------------
// Assertion helpers
// --------------------------------------------------------------------------

// assertExitCode checks the exit code of a result.
func (e *testEnv) assertExitCode(r result, expected int) {
	e.t.Helper()
	if r.ExitCode != expected {
		e.t.Errorf("expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			expected, r.ExitCode, r.Stdout, r.Stderr)
	}
}

// assertContains checks that stdout or stderr contains a substring.
func (e *testEnv) assertContains(r result, substr string) {
	e.t.Helper()
	combined := r.Stdout + r.Stderr
	if !strings.Contains(combined, substr) {
		e.t.Errorf("expected output to contain %q\nstdout: %s\nstderr: %s",
			substr, r.Stdout, r.Stderr)
	}
}

// assertNotContains checks that output does NOT contain a substring.
func (e *testEnv) assertNotContains(r result, substr string) {
	e.t.Helper()
	combined := r.Stdout + r.Stderr
	if strings.Contains(combined, substr) {
		e.t.Errorf("expected output NOT to contain %q\nstdout: %s\nstderr: %s",
			substr, r.Stdout, r.Stderr)
	}
}

// assertStdoutContains checks that stdout specifically contains a substring.
func (e *testEnv) assertStdoutContains(r result, substr string) {
	e.t.Helper()
	if !strings.Contains(r.Stdout, substr) {
		e.t.Errorf("expected stdout to contain %q\ngot: %s", substr, r.Stdout)
	}
}

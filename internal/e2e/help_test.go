package e2e

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("--version")
	e.assertExitCode(r, 0)
	if !strings.Contains(r.Stdout, "gitm version") {
		t.Errorf("expected 'gitm version' in output, got: %s", r.Stdout)
	}
	e.assertStdoutContains(r, "e2e-test")
}

func TestHelp_RootCommand(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("--help")
	e.assertExitCode(r, 0)

	expectedCommands := []string{
		"branch", "checkout", "commit", "discard",
		"repo", "reset", "stash", "status",
		"track", "untrack", "update", "upgrade",
	}
	for _, cmd := range expectedCommands {
		if !strings.Contains(r.Stdout, cmd) {
			t.Errorf("help output missing command: %s", cmd)
		}
	}
}

func TestHelp_SubCommands(t *testing.T) {
	e := newTestEnv(t)

	commands := []string{
		"repo", "branch", "checkout", "commit",
		"discard", "stash", "status", "track",
		"untrack", "update", "reset", "upgrade",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			r := e.runGitm(cmd, "--help")
			e.assertExitCode(r, 0)
			// Each help should contain "Usage:" section
			if !strings.Contains(r.Stdout, "Usage:") {
				t.Errorf("gitm %s --help missing 'Usage:' section", cmd)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	e := newTestEnv(t)

	r := e.runGitm("foobar-unknown")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for unknown command")
	}
	e.assertContains(r, "unknown")
}

// Only test that it runs without crashing. Don't actually upgrade.

func TestUpgrade_SkipsDBInit(t *testing.T) {
	e := newTestEnv(t)

	// Upgrade should work even without any DB initialization
	// (it's in the skip list for PersistentPreRunE).
	r := e.runGitm("upgrade", "--help")
	e.assertExitCode(r, 0)

	combined := r.Stdout + r.Stderr
	if strings.Contains(combined, "database") || strings.Contains(combined, "gitm.db") {
		t.Error("upgrade should not require database initialization")
	}
}

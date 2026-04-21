package cli

import (
	"testing"
)

// TestCommitCmdExists verifies the commit command is created.
func TestCommitCmdExists(t *testing.T) {
	cmd := commitCmd()
	if cmd == nil {
		t.Fatal("commitCmd() returned nil")
	}
}

// TestCommitCmdHasUse verifies the command has the correct Use field.
func TestCommitCmdHasUse(t *testing.T) {
	cmd := commitCmd()
	if cmd.Use != "commit" {
		t.Errorf("commitCmd Use = %q, want %q", cmd.Use, "commit")
	}
}

// TestCommitCmdHasShort verifies the command has a short description.
func TestCommitCmdHasShort(t *testing.T) {
	cmd := commitCmd()
	if cmd.Short == "" {
		t.Error("commitCmd has empty Short description")
	}
}

// TestCommitCmdHasLong verifies the command has detailed help text.
func TestCommitCmdHasLong(t *testing.T) {
	cmd := commitCmd()
	if cmd.Long == "" {
		t.Error("commitCmd has empty Long description")
	}
}

// TestCommitCmdIsRunnable verifies the command has a RunE function.
func TestCommitCmdIsRunnable(t *testing.T) {
	cmd := commitCmd()
	if cmd.RunE == nil {
		t.Error("commitCmd has no RunE function")
	}
}

// TestCommitCmdFlags verifies all expected flags are registered on commit.
func TestCommitCmdFlags(t *testing.T) {
	cmd := commitCmd()

	flags := []struct {
		long      string
		wantShort string
	}{
		{long: "no-push", wantShort: ""},
		{long: "repo", wantShort: "r"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.long)
		if flag == nil {
			t.Errorf("flag --%s not found on commit", f.long)
			continue
		}
		if flag.Shorthand != f.wantShort {
			t.Errorf("flag --%s: expected shorthand %q, got %q", f.long, f.wantShort, flag.Shorthand)
		}
	}
}

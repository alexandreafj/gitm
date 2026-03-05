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

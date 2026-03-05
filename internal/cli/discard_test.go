package cli

import (
	"testing"
)

// TestDiscardCmdExists verifies the discard command is created.
func TestDiscardCmdExists(t *testing.T) {
	cmd := discardCmd()
	if cmd == nil {
		t.Fatal("discardCmd() returned nil")
	}
}

// TestDiscardCmdHasUse verifies the command has the correct Use field.
func TestDiscardCmdHasUse(t *testing.T) {
	cmd := discardCmd()
	if cmd.Use != "discard" {
		t.Errorf("discardCmd Use = %q, want %q", cmd.Use, "discard")
	}
}

// TestDiscardCmdHasShort verifies the command has a short description.
func TestDiscardCmdHasShort(t *testing.T) {
	cmd := discardCmd()
	if cmd.Short == "" {
		t.Error("discardCmd has empty Short description")
	}
}

// TestDiscardCmdHasLong verifies the command has detailed help text.
func TestDiscardCmdHasLong(t *testing.T) {
	cmd := discardCmd()
	if cmd.Long == "" {
		t.Error("discardCmd has empty Long description")
	}
}

// TestDiscardCmdIsRunnable verifies the command has a RunE function.
func TestDiscardCmdIsRunnable(t *testing.T) {
	cmd := discardCmd()
	if cmd.RunE == nil {
		t.Error("discardCmd has no RunE function")
	}
}

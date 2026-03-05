package cli

import (
	"testing"
)

// TestUpdateCmdExists verifies the update command is created.
func TestUpdateCmdExists(t *testing.T) {
	cmd := updateCmd()
	if cmd == nil {
		t.Fatal("updateCmd() returned nil")
	}
}

// TestUpdateCmdHasUse verifies the command has the correct Use field.
func TestUpdateCmdHasUse(t *testing.T) {
	cmd := updateCmd()
	if cmd.Use != "update" {
		t.Errorf("updateCmd Use = %q, want %q", cmd.Use, "update")
	}
}

// TestUpdateCmdHasShort verifies the command has a short description.
func TestUpdateCmdHasShort(t *testing.T) {
	cmd := updateCmd()
	if cmd.Short == "" {
		t.Error("updateCmd has empty Short description")
	}
}

// TestUpdateCmdIsRunnable verifies the command has a RunE function.
func TestUpdateCmdIsRunnable(t *testing.T) {
	cmd := updateCmd()
	if cmd.RunE == nil {
		t.Error("updateCmd has no RunE function")
	}
}

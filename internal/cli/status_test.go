package cli

import (
	"testing"
)

// TestStatusCmdExists verifies the status command is created.
func TestStatusCmdExists(t *testing.T) {
	cmd := statusCmd()
	if cmd == nil {
		t.Fatal("statusCmd() returned nil")
	}
}

// TestStatusCmdHasUse verifies the command has the correct Use field.
func TestStatusCmdHasUse(t *testing.T) {
	cmd := statusCmd()
	if cmd.Use != "status" {
		t.Errorf("statusCmd Use = %q, want %q", cmd.Use, "status")
	}
}

// TestStatusCmdHasShort verifies the command has a short description.
func TestStatusCmdHasShort(t *testing.T) {
	cmd := statusCmd()
	if cmd.Short == "" {
		t.Error("statusCmd has empty Short description")
	}
}

// TestStatusCmdIsRunnable verifies the command has a RunE function.
func TestStatusCmdIsRunnable(t *testing.T) {
	cmd := statusCmd()
	if cmd.RunE == nil {
		t.Error("statusCmd has no RunE function")
	}
}

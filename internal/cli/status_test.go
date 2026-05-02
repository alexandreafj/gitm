package cli

import (
	"testing"
)

func TestStatusCmdExists(t *testing.T) {
	cmd := statusCmd()
	if cmd == nil {
		t.Fatal("statusCmd() returned nil")
	}
}

func TestStatusCmdHasUse(t *testing.T) {
	cmd := statusCmd()
	if cmd.Use != "status" {
		t.Errorf("statusCmd Use = %q, want %q", cmd.Use, "status")
	}
}

func TestStatusCmdHasShort(t *testing.T) {
	cmd := statusCmd()
	if cmd.Short == "" {
		t.Error("statusCmd has empty Short description")
	}
}

func TestStatusCmdIsRunnable(t *testing.T) {
	cmd := statusCmd()
	if cmd.RunE == nil {
		t.Error("statusCmd has no RunE function")
	}
}

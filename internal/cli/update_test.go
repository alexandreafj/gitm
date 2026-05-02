package cli

import (
	"testing"
)

func TestUpdateCmdExists(t *testing.T) {
	cmd := updateCmd()
	if cmd == nil {
		t.Fatal("updateCmd() returned nil")
	}
}

func TestUpdateCmdHasUse(t *testing.T) {
	cmd := updateCmd()
	if cmd.Use != "update" {
		t.Errorf("updateCmd Use = %q, want %q", cmd.Use, "update")
	}
}

func TestUpdateCmdHasShort(t *testing.T) {
	cmd := updateCmd()
	if cmd.Short == "" {
		t.Error("updateCmd has empty Short description")
	}
}

func TestUpdateCmdIsRunnable(t *testing.T) {
	cmd := updateCmd()
	if cmd.RunE == nil {
		t.Error("updateCmd has no RunE function")
	}
}

func TestUpdateCmdHasRepoFlag(t *testing.T) {
	cmd := updateCmd()
	f := cmd.Flags().Lookup("repo")
	if f == nil {
		t.Fatal("updateCmd missing --repo flag")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

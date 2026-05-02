package cli

import (
	"testing"
)

func TestDiscardCmdExists(t *testing.T) {
	cmd := discardCmd()
	if cmd == nil {
		t.Fatal("discardCmd() returned nil")
	}
}

func TestDiscardCmdHasUse(t *testing.T) {
	cmd := discardCmd()
	if cmd.Use != "discard" {
		t.Errorf("discardCmd Use = %q, want %q", cmd.Use, "discard")
	}
}

func TestDiscardCmdHasShort(t *testing.T) {
	cmd := discardCmd()
	if cmd.Short == "" {
		t.Error("discardCmd has empty Short description")
	}
}

func TestDiscardCmdHasLong(t *testing.T) {
	cmd := discardCmd()
	if cmd.Long == "" {
		t.Error("discardCmd has empty Long description")
	}
}

func TestDiscardCmdIsRunnable(t *testing.T) {
	cmd := discardCmd()
	if cmd.RunE == nil {
		t.Error("discardCmd has no RunE function")
	}
}

func TestDiscardCmdHasRepoFlag(t *testing.T) {
	cmd := discardCmd()
	f := cmd.Flags().Lookup("repo")
	if f == nil {
		t.Fatal("discardCmd missing --repo flag")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

func TestDiscardCmdHasExample(t *testing.T) {
	cmd := discardCmd()
	if cmd.Example == "" {
		t.Error("discardCmd has empty Example")
	}
}

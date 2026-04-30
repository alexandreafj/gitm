package cli

import (
	"testing"
)

// TestRootCommandExists verifies that the root command is created without error.
func TestRootCommandExists(t *testing.T) {
	cmd := Root("test")
	if cmd == nil {
		t.Fatal("Root() returned nil")
	}
}

// TestRootCommandHasUse verifies the root command has the correct Use field.
func TestRootCommandHasUse(t *testing.T) {
	cmd := Root("test")
	if cmd.Use != "gitm" {
		t.Errorf("Root command Use = %q, want %q", cmd.Use, "gitm")
	}
}

// TestRootCommandHasShort verifies the root command has a Short description.
func TestRootCommandHasShort(t *testing.T) {
	cmd := Root("test")
	if cmd.Short == "" {
		t.Error("Root command Short is empty")
	}
}

// TestRootCommandHasSubcommands verifies the root command has subcommands registered.
func TestRootCommandHasSubcommands(t *testing.T) {
	cmd := Root("test")
	if len(cmd.Commands()) == 0 {
		t.Error("Root command has no subcommands")
	}
}

// TestRootCommandSubcommandNames verifies expected subcommands exist.
func TestRootCommandSubcommandNames(t *testing.T) {
	cmd := Root("test")
	expectedCommands := []string{"repo", "checkout", "branch", "status", "update", "discard", "commit", "stash", "reset", "track", "untrack", "upgrade"}

	actual := make(map[string]bool)
	for _, sc := range cmd.Commands() {
		actual[sc.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !actual[expected] {
			t.Errorf("subcommand %q not found", expected)
		}
	}
}

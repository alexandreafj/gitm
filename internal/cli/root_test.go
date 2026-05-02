package cli

import (
	"testing"
)

func TestRootCommandExists(t *testing.T) {
	cmd := Root("test")
	if cmd == nil {
		t.Fatal("Root() returned nil")
	}
}

func TestRootCommandHasUse(t *testing.T) {
	cmd := Root("test")
	if cmd.Use != "gitm" {
		t.Errorf("Root command Use = %q, want %q", cmd.Use, "gitm")
	}
}

func TestRootCommandHasShort(t *testing.T) {
	cmd := Root("test")
	if cmd.Short == "" {
		t.Error("Root command Short is empty")
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	cmd := Root("test")
	if len(cmd.Commands()) == 0 {
		t.Error("Root command has no subcommands")
	}
}

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

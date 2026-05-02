package cli

import (
	"testing"
)

func TestStashCmdExists(t *testing.T) {
	cmd := stashCmd()
	if cmd == nil {
		t.Fatal("stashCmd() returned nil")
	}
}

func TestStashCmdHasSubcommands(t *testing.T) {
	cmd := stashCmd()
	if len(cmd.Commands()) == 0 {
		t.Error("stash command has no subcommands")
	}

	// Note: "push" is the default action (RunE), not a subcommand
	expectedSubcommands := []string{"pop", "apply", "list"}
	actual := make(map[string]bool)
	for _, sc := range cmd.Commands() {
		actual[sc.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !actual[expected] {
			t.Errorf("subcommand %q not found in stash", expected)
		}
	}
}

// Note: stashPushCmd is not exported, only the subcommand runner is available.
// This is by design in the CLI structure.

func TestStashPopCmdExists(t *testing.T) {
	cmd := stashPopCmd()
	if cmd == nil {
		t.Fatal("stashPopCmd() returned nil")
	}
}

func TestStashApplyCmdExists(t *testing.T) {
	cmd := stashApplyCmd()
	if cmd == nil {
		t.Fatal("stashApplyCmd() returned nil")
	}
}

func TestStashListCmdExists(t *testing.T) {
	cmd := stashListCmd()
	if cmd == nil {
		t.Fatal("stashListCmd() returned nil")
	}
}

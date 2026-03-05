package cli

import (
	"testing"
)

// TestBranchCmdExists verifies the branch command is created.
func TestBranchCmdExists(t *testing.T) {
	cmd := branchCmd()
	if cmd == nil {
		t.Fatal("branchCmd() returned nil")
	}
}

// TestBranchCmdHasSubcommands verifies branch has subcommands.
func TestBranchCmdHasSubcommands(t *testing.T) {
	cmd := branchCmd()
	if len(cmd.Commands()) == 0 {
		t.Error("branch command has no subcommands")
	}

	// Verify specific subcommands exist
	expectedSubcommands := []string{"create", "rename"}
	actual := make(map[string]bool)
	for _, sc := range cmd.Commands() {
		actual[sc.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !actual[expected] {
			t.Errorf("subcommand %q not found in branch", expected)
		}
	}
}

// TestBranchCreateCmdExists verifies the create subcommand exists.
func TestBranchCreateCmdExists(t *testing.T) {
	cmd := branchCreateCmd()
	if cmd == nil {
		t.Fatal("branchCreateCmd() returned nil")
	}

	if cmd.Use == "" {
		t.Error("branchCreateCmd has empty Use")
	}
}

// TestBranchRenameCmdExists verifies the rename subcommand exists.
func TestBranchRenameCmdExists(t *testing.T) {
	cmd := branchRenameCmd()
	if cmd == nil {
		t.Fatal("branchRenameCmd() returned nil")
	}

	if cmd.Use == "" {
		t.Error("branchRenameCmd has empty Use")
	}
}

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

// TestBranchCreateCmdFlags verifies all expected flags are registered on branch create.
func TestBranchCreateCmdFlags(t *testing.T) {
	cmd := branchCreateCmd()

	flags := []struct {
		long  string
		short string
	}{
		{"all", "a"},
		{"from", "f"},
		{"repo", "r"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.long)
		if flag == nil {
			t.Errorf("flag --%s not found on branch create", f.long)
			continue
		}
		if flag.Shorthand != f.short {
			t.Errorf("flag --%s: expected shorthand -%s, got -%s", f.long, f.short, flag.Shorthand)
		}
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

// TestBranchRenameCmdFlags verifies all expected flags are registered on branch rename.
func TestBranchRenameCmdFlags(t *testing.T) {
	cmd := branchRenameCmd()

	flags := []struct {
		long  string
		short string
	}{
		{"all", "a"},
		{"no-remote", ""},
		{"repo", "r"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.long)
		if flag == nil {
			t.Errorf("flag --%s not found on branch rename", f.long)
			continue
		}
		if flag.Shorthand != f.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", f.long, f.short, flag.Shorthand)
		}
	}
}

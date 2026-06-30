package cli

import "testing"

func TestPushCmdExists(t *testing.T) {
	if pushCmd() == nil {
		t.Fatal("pushCmd() returned nil")
	}
}

func TestPushCmdRegisteredOnRoot(t *testing.T) {
	root := Root("test")
	for _, c := range root.Commands() {
		if c.Name() == "push" {
			return
		}
	}
	t.Error("push command is not registered on root")
}

func TestPushCmdHasUse(t *testing.T) {
	if cmd := pushCmd(); cmd.Use != "push" {
		t.Errorf("pushCmd Use = %q, want %q", cmd.Use, "push")
	}
}

func TestPushCmdHasShort(t *testing.T) {
	if pushCmd().Short == "" {
		t.Error("pushCmd has empty Short description")
	}
}

func TestPushCmdIsRunnable(t *testing.T) {
	if pushCmd().RunE == nil {
		t.Error("pushCmd has no RunE function")
	}
}

func TestPushCmdRejectsArgs(t *testing.T) {
	cmd := pushCmd()
	if err := cmd.Args(cmd, []string{"unexpected"}); err == nil {
		t.Error("pushCmd should reject positional arguments")
	}
}

func TestPushCmdHasRepoFlag(t *testing.T) {
	f := pushCmd().Flags().Lookup("repo")
	if f == nil {
		t.Fatal("pushCmd missing --repo flag")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

func TestPushCmdHasAllFlag(t *testing.T) {
	f := pushCmd().Flags().Lookup("all")
	if f == nil {
		t.Fatal("pushCmd missing --all flag")
	}
	if f.Shorthand != "a" {
		t.Errorf("--all shorthand = %q, want %q", f.Shorthand, "a")
	}
}

func TestPushCmdHasGroupFlag(t *testing.T) {
	if pushCmd().Flags().Lookup("group") == nil {
		t.Fatal("pushCmd missing --group flag")
	}
}

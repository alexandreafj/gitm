package cli

import (
	"testing"
)

// TestCheckoutCmdExists verifies the checkout command is created.
func TestCheckoutCmdExists(t *testing.T) {
	cmd := checkoutCmd()
	if cmd == nil {
		t.Fatal("checkoutCmd() returned nil")
	}
}

// TestCheckoutCmdHasUse verifies the command has the correct Use field.
func TestCheckoutCmdHasUse(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.Use == "" {
		t.Error("checkoutCmd has empty Use")
	}
}

// TestCheckoutCmdHasShort verifies the command has a short description.
func TestCheckoutCmdHasShort(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.Short == "" {
		t.Error("checkoutCmd has empty Short")
	}
}

// TestCheckoutCmdIsRunnable verifies the command is runnable.
func TestCheckoutCmdIsRunnable(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.RunE == nil {
		t.Error("checkoutCmd has no RunE function")
	}
}

// TestCheckoutCmdHasRepoFlag verifies the --repo flag exists with -r shorthand.
func TestCheckoutCmdHasRepoFlag(t *testing.T) {
	cmd := checkoutCmd()
	f := cmd.Flags().Lookup("repo")
	if f == nil {
		t.Fatal("checkoutCmd missing --repo flag")
	}
	if f.Shorthand != "r" {
		t.Errorf("--repo shorthand = %q, want %q", f.Shorthand, "r")
	}
}

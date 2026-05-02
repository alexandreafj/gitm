package cli

import (
	"testing"
)

func TestCheckoutCmdExists(t *testing.T) {
	cmd := checkoutCmd()
	if cmd == nil {
		t.Fatal("checkoutCmd() returned nil")
	}
}

func TestCheckoutCmdHasUse(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.Use == "" {
		t.Error("checkoutCmd has empty Use")
	}
}

func TestCheckoutCmdHasShort(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.Short == "" {
		t.Error("checkoutCmd has empty Short")
	}
}

func TestCheckoutCmdIsRunnable(t *testing.T) {
	cmd := checkoutCmd()
	if cmd.RunE == nil {
		t.Error("checkoutCmd has no RunE function")
	}
}

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

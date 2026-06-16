package cli

import (
	"testing"
)

func TestDoctorCmdExists(t *testing.T) {
	cmd := doctorCmd()
	if cmd == nil {
		t.Fatal("doctorCmd() returned nil")
	}
}

func TestDoctorCmdHasUse(t *testing.T) {
	cmd := doctorCmd()
	if cmd.Use != "doctor" {
		t.Errorf("doctorCmd Use = %q, want %q", cmd.Use, "doctor")
	}
}

func TestDoctorCmdHasShort(t *testing.T) {
	cmd := doctorCmd()
	if cmd.Short == "" {
		t.Error("doctorCmd has empty Short description")
	}
}

func TestDoctorCmdIsRunnable(t *testing.T) {
	cmd := doctorCmd()
	if cmd.RunE == nil {
		t.Error("doctorCmd has no RunE function")
	}
}

func TestDoctorCmdHasRepoFlag(t *testing.T) {
	cmd := doctorCmd()
	if flag := cmd.Flags().Lookup("repo"); flag == nil {
		t.Fatal("doctorCmd missing --repo flag")
	}
	if flag := cmd.Flags().ShorthandLookup("r"); flag == nil {
		t.Fatal("doctorCmd missing -r shorthand")
	}
}

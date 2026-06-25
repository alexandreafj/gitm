package tui

import (
	"fmt"
	"testing"
)

func TestRenderStatus_ConflictUsesConflictStyle(t *testing.T) {
	for _, status := range []string{"UU", "AU", "UD", "UA", "DU"} {
		got := renderStatus(status)
		want := statusConflictStyle.Render(fmt.Sprintf("%-2s", status))
		if got != want {
			t.Errorf("renderStatus(%q) = %q, want conflict style %q", status, got, want)
		}
	}
}

func TestRenderStatus_KnownStatuses(t *testing.T) {
	statuses := []struct {
		input string
		want  string
	}{
		{" M", statusMStyle.Render(fmt.Sprintf("%-2s", "M"))},
		{"M ", statusMStyle.Render(fmt.Sprintf("%-2s", "M"))},
		{"A ", statusAStyle.Render(fmt.Sprintf("%-2s", "A"))},
		{"D ", statusDStyle.Render(fmt.Sprintf("%-2s", "D"))},
		{"R ", statusRStyle.Render(fmt.Sprintf("%-2s", "R"))},
		{"??", statusUStyle.Render(fmt.Sprintf("%-2s", "??"))},
	}
	for _, tt := range statuses {
		got := renderStatus(tt.input)
		if got != tt.want {
			t.Errorf("renderStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

package cli

import (
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestPrintDryRunPreview(t *testing.T) {
	repo := &db.Repository{Alias: "repo1", Path: "/tmp/repo1"}

	output := captureOutput(t, func() {
		printDryRunPreview("Planned work", []dryRunItem{
			{
				repo:    repo,
				actions: []string{"git checkout main", "git pull --ff-only"},
				warning: "checkout conflicts cannot be predicted without running git checkout",
			},
			{
				repo:       &db.Repository{Alias: "repo2", Path: "/tmp/repo2"},
				skipReason: "branch not found",
			},
		})
	})

	for _, want := range []string{
		"DRY RUN: no changes made",
		"Planned work",
		"repo1",
		"git checkout main",
		"checkout conflicts cannot be predicted",
		"repo2",
		"SKIP: branch not found",
		"No changes made.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}

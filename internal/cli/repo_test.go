package cli

import (
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

// ─── TestPrintRepoTable ─────────────────────────────────────────────────────

// TestPrintRepoTableHandlesEmpty verifies that printRepoTable doesn't panic with empty list.
func TestPrintRepoTableHandlesEmpty(t *testing.T) {
	// This test ensures the function doesn't crash with an empty repository list.
	// We can't easily test the actual output without capturing stdout.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRepoTable panicked with empty repos: %v", r)
		}
	}()

	repos := []*db.Repository{}
	printRepoTable(repos)
}

// TestPrintRepoTableHandlesMultipleRepos verifies the function handles multiple repos.
func TestPrintRepoTableHandlesMultipleRepos(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRepoTable panicked with multiple repos: %v", r)
		}
	}()

	repos := []*db.Repository{
		{
			ID:            1,
			Name:          "api",
			Alias:         "api-gateway",
			Path:          "/home/user/api",
			DefaultBranch: "main",
		},
		{
			ID:            2,
			Name:          "web",
			Alias:         "web-ui",
			Path:          "/home/user/web",
			DefaultBranch: "master",
		},
	}
	printRepoTable(repos)
}

func TestRepoAddCmdAliasValidation(t *testing.T) {
	cmd := repoAddCmd()
	cmd.Flags().Set("alias", "alias")
	if err := cmd.RunE(cmd, []string{"/tmp/a", "/tmp/b"}); err == nil {
		t.Fatal("expected error when alias used with multiple paths")
	}
}

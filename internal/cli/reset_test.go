package cli

import (
	"strings"
	"testing"

	"github.com/alexandreferreira/gitm/internal/db"
)

// ─── determineResetMode ──────────────────────────────────────────────────────

func TestDetermineResetMode(t *testing.T) {
	tests := []struct {
		name        string
		soft        bool
		hard        bool
		wantMode    resetMode
		wantErr     bool
		errContains string
	}{
		{
			name:     "no flags gives mixed (default)",
			soft:     false,
			hard:     false,
			wantMode: resetModeMixed,
		},
		{
			name:     "soft flag gives soft mode",
			soft:     true,
			hard:     false,
			wantMode: resetModeSoft,
		},
		{
			name:     "hard flag gives hard mode",
			soft:     false,
			hard:     true,
			wantMode: resetModeHard,
		},
		{
			name:        "both flags is an error",
			soft:        true,
			hard:        true,
			wantErr:     true,
			errContains: "mutually exclusive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := determineResetMode(tc.soft, tc.hard)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantMode {
				t.Errorf("mode = %v, want %v", got, tc.wantMode)
			}
		})
	}
}

// ─── resetModeName ───────────────────────────────────────────────────────────

func TestResetModeName(t *testing.T) {
	tests := []struct {
		mode resetMode
		want string
	}{
		{resetModeMixed, "mixed"},
		{resetModeSoft, "soft"},
		{resetModeHard, "hard"},
	}

	for _, tc := range tests {
		if got := resetModeName(tc.mode); got != tc.want {
			t.Errorf("resetModeName(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// ─── resetModeDescription ────────────────────────────────────────────────────

func TestResetModeDescription(t *testing.T) {
	tests := []struct {
		mode        resetMode
		mustContain string // a key phrase that must appear in the description
	}{
		{resetModeMixed, "unstaged"},
		{resetModeSoft, "staged"},
		{resetModeHard, "DISCARDED"},
	}

	for _, tc := range tests {
		desc := resetModeDescription(tc.mode)
		if !strings.Contains(desc, tc.mustContain) {
			t.Errorf("description for mode %v = %q, want it to contain %q",
				tc.mode, desc, tc.mustContain)
		}
	}
}

// ─── buildResetRef ───────────────────────────────────────────────────────────

func TestBuildResetRef(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "HEAD~1"},
		{2, "HEAD~2"},
		{10, "HEAD~10"},
	}

	for _, tc := range tests {
		if got := buildResetRef(tc.n); got != tc.want {
			t.Errorf("buildResetRef(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ─── buildResetResultMessage ─────────────────────────────────────────────────

func TestBuildResetResultMessage(t *testing.T) {
	info := &repoResetInfo{
		repo: &db.Repository{Alias: "my-repo", Path: "/tmp/repo"},
		commits: []string{
			"abc1234 add feature",
			"def5678 fix bug",
		},
	}

	t.Run("mixed mode mentions unstaged", func(t *testing.T) {
		msg := buildResetResultMessage(info, resetModeMixed)
		if !strings.Contains(msg, "mixed") {
			t.Errorf("expected 'mixed' in message, got: %q", msg)
		}
		if !strings.Contains(msg, "unstaged") {
			t.Errorf("expected 'unstaged' in message, got: %q", msg)
		}
		if !strings.Contains(msg, "add feature") {
			t.Errorf("expected commit subject in message, got: %q", msg)
		}
	})

	t.Run("soft mode mentions staged", func(t *testing.T) {
		msg := buildResetResultMessage(info, resetModeSoft)
		if !strings.Contains(msg, "soft") {
			t.Errorf("expected 'soft' in message, got: %q", msg)
		}
		if !strings.Contains(msg, "staged") {
			t.Errorf("expected 'staged' in message, got: %q", msg)
		}
	})

	t.Run("hard mode mentions discarded", func(t *testing.T) {
		msg := buildResetResultMessage(info, resetModeHard)
		if !strings.Contains(msg, "hard") {
			t.Errorf("expected 'hard' in message, got: %q", msg)
		}
		if !strings.Contains(msg, "discard") {
			t.Errorf("expected 'discard' in message, got: %q", msg)
		}
	})

	t.Run("commit count is included", func(t *testing.T) {
		msg := buildResetResultMessage(info, resetModeMixed)
		if !strings.Contains(msg, "2 commit") {
			t.Errorf("expected commit count in message, got: %q", msg)
		}
	})

	t.Run("nil info returns safe fallback", func(t *testing.T) {
		msg := buildResetResultMessage(nil, resetModeSoft)
		if msg == "" {
			t.Error("expected non-empty fallback message for nil info")
		}
	})
}

// ─── gatherResetInfo ─────────────────────────────────────────────────────────

func TestGatherResetInfoSkipsReposWithTooFewCommits(t *testing.T) {
	// We inject a repo with a path that does not exist — git will fail,
	// which means CommitLog fails, which triggers the "skipped" path.
	repos := []*db.Repository{
		{Alias: "ghost", Path: "/nonexistent/path/that/will/never/exist"},
	}

	infos, skipped := gatherResetInfo(repos, 3, "HEAD~3")

	if len(infos) != 0 {
		t.Errorf("expected no valid infos for an invalid repo, got %d", len(infos))
	}
	if len(skipped) == 0 {
		t.Error("expected at least one skipped message for an invalid repo path")
	}
}

package cli

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/alexandreafj/gitm/internal/db"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var errFakeInspect = errors.New("read current branch: not a git repository")

// forceColor turns colored output on for the duration of a test. Colors are
// normally disabled when stdout is not a terminal, which is exactly when the
// alignment bug this guards against would go unnoticed.
func forceColor(t *testing.T) {
	t.Helper()
	previous := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = previous })
}

// columnStarts returns the visible rune offset at which each column of a
// rendered row begins. ANSI sequences are stripped first because they occupy
// no width on screen; two or more spaces separate columns.
func columnStarts(line string) []int {
	runes := []rune(ansiPattern.ReplaceAllString(line, ""))
	var starts []int
	inGap := true
	for i := 0; i < len(runes); i++ {
		if runes[i] == ' ' {
			if i+1 < len(runes) && runes[i+1] == ' ' {
				inGap = true
			}
			continue
		}
		if inGap {
			starts = append(starts, i)
			inGap = false
		}
	}
	return starts
}

// assertAlignedColumns fails unless every row of the table starts its columns
// at the same visible offsets. Rows are compared from the right, because a
// leading marker column (the active context, the checked-out target branch) is
// blank on most rows. Lines after the first blank one are trailer text.
func assertAlignedColumns(t *testing.T, output string, wantColumns int) {
	t.Helper()

	var lines []string
	var starts [][]int
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if line == "" {
			break
		}
		lines = append(lines, line)
		starts = append(starts, columnStarts(line))
	}
	if len(lines) == 0 {
		t.Fatal("no table rows in output")
	}

	reference := starts[0]
	for _, s := range starts {
		if len(s) > len(reference) {
			reference = s
		}
	}
	if len(reference) != wantColumns {
		t.Fatalf("widest row has %d columns, want %d\n%s", len(reference), wantColumns, output)
	}

	for i, s := range starts {
		tail := reference[len(reference)-len(s):]
		for col := range s {
			if s[col] != tail[col] {
				t.Fatalf("row %d %q column %d starts at %d, want %d", i, lines[i], col, s[col], tail[col])
			}
		}
	}
}

func TestBranchesTableAlignsColoredColumns(t *testing.T) {
	forceColor(t)

	infos := []branchInfo{
		{name: "backend", current: "dev", hasSubject: true, upstream: "origin/dev", countsKnown: true, merged: mergedDefault},
		{name: "graphql-gateway", current: "feature/MA-26729_membership", hasSubject: true, merged: mergedYes},
		{name: "web", current: "main", hasSubject: true, upstream: "origin/main", ahead: 2, behind: 3, countsKnown: true, merged: mergedNo},
	}

	var buf bytes.Buffer
	if err := printBranchesTable(&buf, infos, ""); err != nil {
		t.Fatalf("printBranchesTable: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 5)
}

func TestBranchesTableAlignsErrorRows(t *testing.T) {
	forceColor(t)

	infos := []branchInfo{
		{name: "backend", current: "dev", targetState: "local+remote", onTarget: true, hasSubject: true, upstream: "origin/dev", countsKnown: true},
		{name: "webapp", current: "main", targetState: "missing"},
		{name: "broken", err: errFakeInspect},
	}

	var buf bytes.Buffer
	if err := printBranchesTable(&buf, infos, "feature/x"); err != nil {
		t.Fatalf("printBranchesTable: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 7)
}

func TestContextTableAlignsActiveMarker(t *testing.T) {
	forceColor(t)

	contexts := []*db.Context{
		{Name: db.DefaultContextName, RepoCount: 0},
		{Name: "mittanbud", RepoCount: 2, Active: true},
	}

	var buf bytes.Buffer
	if err := printContextTable(&buf, contexts); err != nil {
		t.Fatalf("printContextTable: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 4)
}

func TestGroupTableAlignsColoredColumns(t *testing.T) {
	forceColor(t)

	groups := []*db.Group{
		{Name: db.DefaultGroupName, RepoCount: 12},
		{Name: "backend-services", RepoCount: 3},
	}

	var buf bytes.Buffer
	if err := printGroupTable(&buf, groups); err != nil {
		t.Fatalf("printGroupTable: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 3)
}

func TestRepoTableAlignsColoredColumns(t *testing.T) {
	forceColor(t)

	repos := []*db.Repository{
		{ID: 1, Alias: "api", Path: "/home/user/api", DefaultBranch: "main"},
		{ID: 2, Alias: "very-long-repository-alias", Path: "/home/user/web", DefaultBranch: "master"},
	}

	var buf bytes.Buffer
	if err := printRepoTable(&buf, repos); err != nil {
		t.Fatalf("printRepoTable: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 4)
}

func TestDoctorTableAlignsColoredColumns(t *testing.T) {
	forceColor(t)

	reports := []doctorReport{
		{repo: &db.Repository{Alias: "api"}},
		{repo: &db.Repository{Alias: "long-alias-here"}, checks: []doctorCheck{
			{severity: doctorError, message: "path does not exist"},
		}},
	}

	var buf bytes.Buffer
	if err := printDoctorReports(&buf, reports); err != nil {
		t.Fatalf("printDoctorReports: %v", err)
	}
	assertAlignedColumns(t, buf.String(), 3)
}

func TestCellReturnsPlainTextWhenColorDisabled(t *testing.T) {
	previous := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = previous })

	if got := cell(aliasColor, "api"); got != "api" {
		t.Fatalf("cell() = %q, want %q", got, "api")
	}
	if got := cell(nil, "api"); got != "api" {
		t.Fatalf("cell(nil) = %q, want %q", got, "api")
	}
}

func TestTruncateCountsRunesNotBytes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter than limit", "main", 10, "main"},
		{"exactly at limit", "main", 4, "main"},
		{"ascii cut", "feature/long-branch", 10, "feature/l…"},
		{"multi-byte kept whole", "føøbar-brønch", 6, "føøba…"},
		{"multi-byte under limit", "brønch", 6, "brønch"},
		{"limit of one", "main", 1, "m"},
		{"zero limit", "main", 0, "main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.max); got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

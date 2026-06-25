package cli

import (
	"strings"
	"testing"
)

// noReposMessage must distinguish "nothing registered at all" from "filters
// matched nothing", so --repo/--group users aren't told to add repositories.
func TestNoReposMessageFilterAware(t *testing.T) {
	if msg := noReposMessage(nil, ""); !strings.Contains(msg, "No repositories registered") {
		t.Fatalf("unfiltered message = %q, want registration hint", msg)
	}
	if msg := noReposMessage([]string{"x"}, ""); !strings.Contains(msg, "filter") {
		t.Fatalf("repo-filtered message = %q, want filter hint", msg)
	}
	if msg := noReposMessage(nil, "backend"); !strings.Contains(msg, "filter") {
		t.Fatalf("group-filtered message = %q, want filter hint", msg)
	}
}

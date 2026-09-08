package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// WorkingTree holds one local porcelain snapshot. Counts are meaningful only
// when TrackingKnown is true; an upstream may be configured but unavailable.
type WorkingTree struct {
	Branch             string
	Upstream           string
	Detached           bool
	Unborn             bool
	ChangedFiles       int
	Conflicts          bool
	Ahead              int
	Behind             int
	TrackingKnown      bool
	TrackingConfigured bool
}

// ReadWorkingTree inspects local state without contacting a remote.
func ReadWorkingTree(path string) (WorkingTree, error) {
	var state WorkingTree
	out, err := run(path, "status", "--porcelain=v2", "--branch", "--ahead-behind", "-z", "--untracked-files=all")
	if err != nil {
		return state, fmt.Errorf("read working tree: %w", err)
	}
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		switch {
		case strings.HasPrefix(record, "# branch.head "):
			state.Branch = strings.TrimPrefix(record, "# branch.head ")
			state.Detached = state.Branch == "(detached)"
		case record == "# branch.oid (initial)":
			state.Unborn = true
		case strings.HasPrefix(record, "# branch.upstream "):
			state.Upstream = strings.TrimPrefix(record, "# branch.upstream ")
		case strings.HasPrefix(record, "# branch.ab "):
			if _, err := fmt.Sscanf(record, "# branch.ab +%d -%d", &state.Ahead, &state.Behind); err != nil {
				return state, fmt.Errorf("parse tracking counts: %w", err)
			}
			state.TrackingKnown = true
		case strings.HasPrefix(record, "2 "):
			i++ // Rename records include a second NUL-delimited path.
			if i >= len(records) || records[i] == "" {
				return state, fmt.Errorf("missing original rename path")
			}
			state.ChangedFiles++
		case strings.HasPrefix(record, "1 "), strings.HasPrefix(record, "? "):
			state.ChangedFiles++
		case strings.HasPrefix(record, "u "):
			state.ChangedFiles++
			state.Conflicts = true
		}
	}
	if state.Branch == "" {
		return state, fmt.Errorf("working tree status is missing branch information")
	}
	state.TrackingConfigured = state.Upstream != ""
	if !state.TrackingConfigured && !state.Detached && !state.Unborn {
		state.TrackingConfigured, err = hasTrackingConfiguration(path, state.Branch)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

func hasTrackingConfiguration(path, branch string) (bool, error) {
	if _, err := run(path, "config", "--get", "branch."+branch+".merge"); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("read tracking configuration for %s: %w", branch, err)
	}
	return true, nil
}

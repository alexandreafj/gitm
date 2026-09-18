package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/alexandreafj/gitm/internal/git"
)

type repoStatus struct {
	Alias        string   `json:"alias"`
	Path         string   `json:"path"`
	Branch       string   `json:"branch"`
	Dirty        *bool    `json:"dirty"`
	ChangedFiles *int     `json:"changed_files"`
	Upstream     string   `json:"upstream"`
	Ahead        *int     `json:"ahead"`
	Behind       *int     `json:"behind"`
	RemoteState  string   `json:"remote_state"`
	Detached     bool     `json:"detached"`
	Unborn       bool     `json:"unborn"`
	Conflicts    bool     `json:"conflicts"`
	Operations   []string `json:"operations"`
	Error        string   `json:"error"`
}

func collectRepoStatus(repo *db.Repository, fetch bool) repoStatus {
	s := repoStatus{Alias: repo.Alias, Path: repo.Path, RemoteState: "unknown", Operations: []string{}}
	var inspectionErr error
	if fetch {
		if err := git.Fetch(repo.Path); err != nil {
			inspectionErr = fmt.Errorf("fetch failed: %w", err)
		}
	}
	refreshFailed := inspectionErr != nil
	state, err := git.ReadWorkingTree(repo.Path)
	if err != nil {
		s.Error = errors.Join(inspectionErr, err).Error()
		return s
	}
	s.Branch, s.Upstream = state.Branch, state.Upstream
	s.Detached, s.Unborn, s.Conflicts = state.Detached, state.Unborn, state.Conflicts
	dirty := state.ChangedFiles > 0
	s.Dirty, s.ChangedFiles = &dirty, &state.ChangedFiles
	if !state.TrackingConfigured {
		s.RemoteState = "none"
	}
	if state.TrackingKnown {
		s.Ahead, s.Behind = &state.Ahead, &state.Behind
		s.RemoteState = "cached"
		if fetch {
			s.RemoteState = "fresh"
		}
	} else if state.TrackingConfigured && !state.Unborn {
		inspectionErr = errors.Join(inspectionErr, fmt.Errorf("configured upstream %q is unavailable", state.Upstream))
	}
	if refreshFailed {
		s.RemoteState = "stale"
	}
	ops, err := git.InProgressOperations(repo.Path)
	if err != nil {
		inspectionErr = errors.Join(inspectionErr, fmt.Errorf("inspect in-progress operations: %w", err))
	} else if len(ops) > 0 {
		s.Operations = ops
	}
	if inspectionErr != nil {
		s.Error = inspectionErr.Error()
	}
	return s
}

func (s repoStatus) needsAttention() bool {
	return s.Error != "" || s.Dirty != nil && *s.Dirty || s.Ahead != nil && *s.Ahead > 0 ||
		s.Behind != nil && *s.Behind > 0 || s.Upstream == "" || s.Detached || s.Unborn || s.Conflicts || len(s.Operations) > 0
}

func renderStatusTable(report statusReport) (string, error) {
	var out strings.Builder
	fmt.Fprintf(&out, "Context: %s\n\n", report.Context)
	if len(report.Repositories) == 0 {
		out.WriteString("No repositories need attention.\n")
		return out.String(), nil
	}
	var rendered strings.Builder
	w := newTable(&rendered)
	headerRow(w, "REPO", "BRANCH", "DIRTY", "REMOTE", "ATTENTION")
	for _, s := range report.Repositories {
		dirty := "unknown"
		if s.Dirty != nil {
			dirty = "clean"
			if *s.Dirty {
				dirty = fmt.Sprintf("%d changed", *s.ChangedFiles)
			}
		}
		remote := "unknown"
		if s.RemoteState == "none" {
			remote = "no upstream"
		}
		if s.Ahead != nil && s.Behind != nil {
			remote = "up to date"
			if *s.Ahead > 0 || *s.Behind > 0 {
				remote = fmt.Sprintf("%d ahead, %d behind", *s.Ahead, *s.Behind)
			}
		}
		if s.RemoteState == "cached" || s.RemoteState == "fresh" || s.RemoteState == "stale" {
			remote += " (" + s.RemoteState + ")"
		}
		attention := append([]string{}, s.Operations...)
		if s.Detached {
			attention = append(attention, "detached HEAD")
		}
		if s.Unborn {
			attention = append(attention, "no commits yet")
		}
		if s.Conflicts {
			attention = append(attention, "conflicts")
		}
		if s.Error != "" {
			attention = append(attention, "ERROR: "+strings.Join(strings.Fields(s.Error), " "))
		}
		row(w, cell(aliasColor, s.Alias), s.Branch, dirty, remote, strings.Join(attention, "; "))
	}
	if err := flushTable(w, "status"); err != nil {
		return "", err
	}
	out.WriteString(rendered.String())
	return out.String(), nil
}

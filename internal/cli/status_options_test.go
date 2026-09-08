package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusJSONAndAttention(t *testing.T) {
	database = setupTestDB(t)
	_, clean := newRepo(t, database, "clean")
	_, dirty := newRepo(t, database, "dirty")
	newRepo(t, database, "local")
	for _, dir := range []string{clean, dirty} {
		mustRunGit(t, dir, "remote", "add", "origin", dir)
		mustRunGit(t, dir, "fetch", "origin")
		mustRunGit(t, dir, "branch", "--set-upstream-to=origin/main", "main")
	}
	writeFile(t, dirty, "space name.txt", "changed")
	cmd := statusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--attention"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Version      int    `json:"version"`
		Context      string `json:"context"`
		Repositories []struct {
			Alias       string `json:"alias"`
			Dirty       *bool  `json:"dirty"`
			Ahead       *int   `json:"ahead"`
			Behind      *int   `json:"behind"`
			RemoteState string `json:"remote_state"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if report.Version != 1 || report.Context != "default" || len(report.Repositories) != 2 {
		t.Fatalf("unexpected report: %s", out.String())
	}
	d, l := report.Repositories[0], report.Repositories[1]
	if d.Alias != "dirty" || d.Dirty == nil || !*d.Dirty || d.Ahead == nil || *d.Ahead != 0 || d.RemoteState != "cached" {
		t.Fatalf("dirty entry: %+v", d)
	}
	if l.Alias != "local" || l.Ahead != nil || l.Behind != nil || l.RemoteState != "none" {
		t.Fatalf("local-only entry: %+v", l)
	}
}

func TestStatusJSONPartialFailure(t *testing.T) {
	database = setupTestDB(t)
	newRepo(t, database, "healthy")
	if _, err := database.AddRepository("missing", "missing", filepath.Join(t.TempDir(), "missing"), "main"); err != nil {
		t.Fatal(err)
	}
	cmd := statusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "--attention"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected partial failure")
	}
	var report struct {
		Repositories []struct {
			Alias string `json:"alias"`
			Dirty *bool  `json:"dirty"`
			Error string `json:"error"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON after failure: %v\n%s", err, out.String())
	}
	if len(report.Repositories) != 2 || report.Repositories[0].Alias != "healthy" || report.Repositories[1].Error == "" || report.Repositories[1].Dirty != nil {
		t.Fatalf("unexpected partial report: %s", out.String())
	}
}

func TestStatusFetchFailureShowsStaleCounts(t *testing.T) {
	database = setupTestDB(t)
	_, dir := newRepo(t, database, "offline")
	mustRunGit(t, dir, "remote", "add", "origin", dir)
	mustRunGit(t, dir, "fetch", "origin")
	mustRunGit(t, dir, "branch", "--set-upstream-to=origin/main", "main")
	mustRunGit(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing"))
	cmd := statusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--fetch", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("failed fetch must fail status")
	}
	var report struct {
		Repositories []struct {
			RemoteState string `json:"remote_state"`
			Ahead       *int   `json:"ahead"`
			Error       string `json:"error"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Repositories) != 1 || report.Repositories[0].RemoteState != "stale" || report.Repositories[0].Ahead == nil || report.Repositories[0].Error == "" {
		t.Fatalf("unexpected stale report: %s", out.String())
	}
}

func TestStatusMissingRepoReturnsError(t *testing.T) {
	database = setupTestDB(t)
	_, dir := newRepo(t, database, "missing")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := runStatus(false, nil); err == nil {
		t.Fatal("missing repository must fail status")
	}
}

func TestStatusJSONEmpty(t *testing.T) {
	database = setupTestDB(t)
	cmd := statusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), `"repositories": []`) {
		t.Fatalf("expected empty JSON array: %s", out.String())
	}
}

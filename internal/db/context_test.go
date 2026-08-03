package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	database, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestDefaultContextExists(t *testing.T) {
	database := openTestDB(t)

	active, err := database.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext: %v", err)
	}
	if active.Name != DefaultContextName {
		t.Fatalf("active context = %q, want %q", active.Name, DefaultContextName)
	}
	if !active.Active {
		t.Fatal("default context should be marked active")
	}
}

func TestCreateContext(t *testing.T) {
	database := openTestDB(t)

	context, err := database.CreateContext("work")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if context.Name != "work" {
		t.Fatalf("context name = %q, want work", context.Name)
	}

	if _, err := database.CreateContext("work"); err == nil {
		t.Fatal("expected error creating duplicate context")
	}
	if _, err := database.CreateContext(DefaultContextName); !errors.Is(err, ErrReservedContext) {
		t.Fatalf("expected ErrReservedContext, got %v", err)
	}
	if _, err := database.CreateContext("has space"); !errors.Is(err, ErrInvalidContextName) {
		t.Fatalf("expected ErrInvalidContextName, got %v", err)
	}
	if _, err := database.CreateContext(""); !errors.Is(err, ErrInvalidContextName) {
		t.Fatalf("expected ErrInvalidContextName, got %v", err)
	}
}

func TestSetActiveContext(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	switched, err := database.SetActiveContext("work")
	if err != nil {
		t.Fatalf("SetActiveContext: %v", err)
	}
	if !switched.Active {
		t.Fatal("switched context should be marked active")
	}

	active, err := database.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext: %v", err)
	}
	if active.Name != "work" {
		t.Fatalf("active context = %q, want work", active.Name)
	}

	if _, err := database.SetActiveContext("missing"); !errors.Is(err, ErrContextNotFound) {
		t.Fatalf("expected ErrContextNotFound, got %v", err)
	}
}

func TestAddRepositoryUsesActiveContext(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	work, err := database.SetActiveContext("work")
	if err != nil {
		t.Fatalf("SetActiveContext: %v", err)
	}

	repo, err := database.AddRepository("api", "api", t.TempDir(), "main")
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	if repo.ContextID != work.ID {
		t.Fatalf("repo context id = %d, want %d", repo.ContextID, work.ID)
	}

	repos, err := database.ListRepositoriesByContext("work")
	if err != nil {
		t.Fatalf("ListRepositoriesByContext: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("work context repos = %v, want [api]", repos)
	}

	defaultRepos, err := database.ListRepositoriesByContext(DefaultContextName)
	if err != nil {
		t.Fatalf("ListRepositoriesByContext default: %v", err)
	}
	if len(defaultRepos) != 0 {
		t.Fatalf("default context should be empty, got %d repos", len(defaultRepos))
	}
}

func TestAssignRepositoriesToContext(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.AddRepository("api", "api", t.TempDir(), "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	work, err := database.CreateContext("work")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}

	if err := database.AssignRepositoriesToContext("work", []string{"api"}); err != nil {
		t.Fatalf("AssignRepositoriesToContext: %v", err)
	}
	repo, err := database.GetRepository("api")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if repo.ContextID != work.ID {
		t.Fatalf("repo context id = %d, want %d", repo.ContextID, work.ID)
	}

	// Moving to another context removes it from the previous one.
	if _, err := database.CreateContext("personal"); err != nil {
		t.Fatalf("CreateContext personal: %v", err)
	}
	if err := database.AssignRepositoriesToContext("personal", []string{"api"}); err != nil {
		t.Fatalf("AssignRepositoriesToContext personal: %v", err)
	}
	workRepos, err := database.ListRepositoriesByContext("work")
	if err != nil {
		t.Fatalf("ListRepositoriesByContext work: %v", err)
	}
	if len(workRepos) != 0 {
		t.Fatalf("work context should be empty after move, got %d repos", len(workRepos))
	}

	if err := database.AssignRepositoriesToContext("missing", []string{"api"}); !errors.Is(err, ErrContextNotFound) {
		t.Fatalf("expected ErrContextNotFound, got %v", err)
	}
	if err := database.AssignRepositoriesToContext("work", []string{"nope"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRenameContext(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if err := database.RenameContext("work", "job"); err != nil {
		t.Fatalf("RenameContext: %v", err)
	}
	if _, err := database.GetContext("job"); err != nil {
		t.Fatalf("GetContext job: %v", err)
	}
	if err := database.RenameContext("missing", "x"); !errors.Is(err, ErrContextNotFound) {
		t.Fatalf("expected ErrContextNotFound, got %v", err)
	}
	if err := database.RenameContext(DefaultContextName, "x"); !errors.Is(err, ErrReservedContext) {
		t.Fatalf("expected ErrReservedContext, got %v", err)
	}
	if err := database.RenameContext("job", DefaultContextName); !errors.Is(err, ErrReservedContext) {
		t.Fatalf("expected ErrReservedContext, got %v", err)
	}
}

func TestDeleteContextMovesReposToDefault(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if _, err := database.AddRepository("api", "api", t.TempDir(), "main"); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	if err := database.AssignRepositoriesToContext("work", []string{"api"}); err != nil {
		t.Fatalf("AssignRepositoriesToContext: %v", err)
	}

	moved, err := database.DeleteContext("work")
	if err != nil {
		t.Fatalf("DeleteContext: %v", err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1", moved)
	}
	repos, err := database.ListRepositoriesByContext(DefaultContextName)
	if err != nil {
		t.Fatalf("ListRepositoriesByContext: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("default context repos = %v, want [api]", repos)
	}

	if _, err := database.DeleteContext(DefaultContextName); !errors.Is(err, ErrReservedContext) {
		t.Fatalf("expected ErrReservedContext, got %v", err)
	}
	if _, err := database.DeleteContext("missing"); !errors.Is(err, ErrContextNotFound) {
		t.Fatalf("expected ErrContextNotFound, got %v", err)
	}
}

func TestDeleteActiveContextRefused(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if _, err := database.SetActiveContext("work"); err != nil {
		t.Fatalf("SetActiveContext: %v", err)
	}
	if _, err := database.DeleteContext("work"); !errors.Is(err, ErrContextActive) {
		t.Fatalf("expected ErrContextActive, got %v", err)
	}
}

func TestActiveContextFallsBackAfterExternalDelete(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if _, err := database.SetActiveContext("work"); err != nil {
		t.Fatalf("SetActiveContext: %v", err)
	}
	// Simulate a dangling active pointer (e.g. row removed by hand).
	if _, err := database.conn.Exec(`DELETE FROM contexts WHERE name = 'work'`); err != nil {
		t.Fatalf("delete context row: %v", err)
	}

	active, err := database.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext: %v", err)
	}
	if active.Name != DefaultContextName {
		t.Fatalf("active context = %q, want %q", active.Name, DefaultContextName)
	}
}

func TestListContexts(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.CreateContext("work"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if _, err := database.CreateContext("personal"); err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	contexts, err := database.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(contexts) != 3 {
		t.Fatalf("len(contexts) = %d, want 3", len(contexts))
	}
	if contexts[0].Name != DefaultContextName {
		t.Fatalf("first context = %q, want %q", contexts[0].Name, DefaultContextName)
	}
	if !contexts[0].Active {
		t.Fatal("default context should be active initially")
	}
}

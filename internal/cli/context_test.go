package cli

import (
	"strings"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestContextCommandRegistered(t *testing.T) {
	root := Root("test")
	for _, sub := range root.Commands() {
		if sub.Name() == "context" {
			expected := []string{"list", "show", "current", "create", "use", "rename", "delete", "add", "remove"}
			actual := make(map[string]bool)
			for _, c := range sub.Commands() {
				actual[c.Name()] = true
			}
			for _, name := range expected {
				if !actual[name] {
					t.Errorf("context subcommand %q not found", name)
				}
			}
			return
		}
	}
	t.Fatal("context command not registered on root")
}

func TestRunContextCreateUseAndCurrent(t *testing.T) {
	setupTestDB(t)

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextCreate("work"); err == nil {
		t.Fatal("expected error creating duplicate context")
	}
	if err := runContextUse("work"); err != nil {
		t.Fatalf("runContextUse: %v", err)
	}

	out := captureOutput(t, func() {
		if err := runContextCurrent(); err != nil {
			t.Errorf("runContextCurrent: %v", err)
		}
	})
	if strings.TrimSpace(out) != "work" {
		t.Fatalf("current context output = %q, want work", strings.TrimSpace(out))
	}

	if err := runContextUse("missing"); err == nil {
		t.Fatal("expected error switching to unknown context")
	}
}

func TestRunContextAddAndShow(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")
	newRepo(t, database, "web")

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextAdd("work", []string{"api"}); err != nil {
		t.Fatalf("runContextAdd: %v", err)
	}

	out := captureOutput(t, func() {
		if err := runContextShow("work"); err != nil {
			t.Errorf("runContextShow: %v", err)
		}
	})
	if !strings.Contains(out, "api") {
		t.Fatalf("context show output missing api:\n%s", out)
	}
	if strings.Contains(out, "web") {
		t.Fatalf("context show output should not contain web:\n%s", out)
	}

	// No argument shows the active (default) context.
	out = captureOutput(t, func() {
		if err := runContextShow(""); err != nil {
			t.Errorf("runContextShow active: %v", err)
		}
	})
	if !strings.Contains(out, db.DefaultContextName) || !strings.Contains(out, "web") {
		t.Fatalf("active context show output unexpected:\n%s", out)
	}

	if err := runContextAdd("missing", []string{"api"}); err == nil {
		t.Fatal("expected error adding to unknown context")
	}
	if err := runContextAdd("work", []string{"nope"}); err == nil {
		t.Fatal("expected error adding unknown repo")
	}
}

func TestRunContextRemove(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")
	newRepo(t, database, "web")

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextAdd("work", []string{"api"}); err != nil {
		t.Fatalf("runContextAdd: %v", err)
	}
	if err := runContextRemove("work", []string{"api"}); err != nil {
		t.Fatalf("runContextRemove: %v", err)
	}
	repo, err := database.GetRepository("api")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	defaultCtx, err := database.GetContext(db.DefaultContextName)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if repo.ContextID != defaultCtx.ID {
		t.Fatalf("repo context id = %d, want default %d", repo.ContextID, defaultCtx.ID)
	}

	// Removing a repo that is not in the context is an error, not a silent move.
	if err := runContextRemove("work", []string{"web"}); err == nil {
		t.Fatal("expected error removing repo that is not in the context")
	}
}

func TestRunContextRenameAndDelete(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextAdd("work", []string{"api"}); err != nil {
		t.Fatalf("runContextAdd: %v", err)
	}
	if err := runContextRename("work", "job"); err != nil {
		t.Fatalf("runContextRename: %v", err)
	}
	if err := runContextRename("default", "x"); err == nil {
		t.Fatal("expected error renaming default context")
	}

	if err := runContextDelete("job"); err != nil {
		t.Fatalf("runContextDelete: %v", err)
	}
	repos, err := database.ListRepositoriesByContext(db.DefaultContextName)
	if err != nil {
		t.Fatalf("ListRepositoriesByContext: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("default context repos = %d, want 1", len(repos))
	}

	if err := runContextDelete("default"); err == nil {
		t.Fatal("expected error deleting default context")
	}
}

func TestRunContextDeleteActiveRefused(t *testing.T) {
	setupTestDB(t)

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextUse("work"); err != nil {
		t.Fatalf("runContextUse: %v", err)
	}
	if err := runContextDelete("work"); err == nil {
		t.Fatal("expected error deleting active context")
	}
}

func TestRunContextList(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	out := captureOutput(t, func() {
		if err := runContextList(); err != nil {
			t.Errorf("runContextList: %v", err)
		}
	})
	for _, want := range []string{"CONTEXT", db.DefaultContextName, "work", "built-in", "custom"} {
		if !strings.Contains(out, want) {
			t.Errorf("context list output missing %q:\n%s", want, out)
		}
	}
}

func TestResolveReposScopedToActiveContext(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")
	newRepo(t, database, "web")

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextAdd("work", []string{"api"}); err != nil {
		t.Fatalf("runContextAdd: %v", err)
	}
	if err := runContextUse("work"); err != nil {
		t.Fatalf("runContextUse: %v", err)
	}

	// No filters: only repos in the active context are returned.
	repos, err := resolveReposWithGroup(nil, "")
	if err != nil {
		t.Fatalf("resolveReposWithGroup: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("resolved repos = %v, want [api]", repoAliases(repos))
	}

	// Explicit alias outside the active context is refused.
	if _, err := resolveReposWithGroup([]string{"web"}, ""); err == nil {
		t.Fatal("expected error resolving repo outside the active context")
	} else if !strings.Contains(err.Error(), "active context") {
		t.Fatalf("error should mention active context, got: %v", err)
	}

	// Explicit alias inside the active context works.
	repos, err = resolveReposWithGroup([]string{"api"}, "")
	if err != nil {
		t.Fatalf("resolveReposWithGroup api: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("resolved repos = %v, want [api]", repoAliases(repos))
	}
}

func TestResolveReposGroupIntersectsActiveContext(t *testing.T) {
	database := setupTestDB(t)
	newRepo(t, database, "api")
	newRepo(t, database, "web")

	// The built-in "all" group spans both repos, but only the active context's
	// members may be returned.
	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextAdd("work", []string{"api"}); err != nil {
		t.Fatalf("runContextAdd: %v", err)
	}
	if err := runContextUse("work"); err != nil {
		t.Fatalf("runContextUse: %v", err)
	}

	repos, err := resolveReposWithGroup(nil, db.DefaultGroupName)
	if err != nil {
		t.Fatalf("resolveReposWithGroup: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("resolved repos = %v, want [api]", repoAliases(repos))
	}
}

func TestRepoAddUsesActiveContext(t *testing.T) {
	database := setupTestDB(t)

	if err := runContextCreate("work"); err != nil {
		t.Fatalf("runContextCreate: %v", err)
	}
	if err := runContextUse("work"); err != nil {
		t.Fatalf("runContextUse: %v", err)
	}

	newRepo(t, database, "api")
	repos, err := database.ListRepositoriesByContext("work")
	if err != nil {
		t.Fatalf("ListRepositoriesByContext: %v", err)
	}
	if len(repos) != 1 || repos[0].Alias != "api" {
		t.Fatalf("work context repos = %v, want [api]", repoAliases(repos))
	}
}

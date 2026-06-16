package cli

import (
	"errors"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
)

func TestResolveReposWithGroup_NoFiltersReturnsAll(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	addRepoRecord(t, "repo2")

	repos, err := resolveReposWithGroup(nil, "")
	if err != nil {
		t.Fatalf("resolveReposWithGroup: %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo1", "repo2"}; !sameAliasSlice(got, want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
}

func TestResolveReposWithGroup_AllGroupReturnsAll(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	addRepoRecord(t, "repo2")

	repos, err := resolveReposWithGroup(nil, db.DefaultGroupName)
	if err != nil {
		t.Fatalf("resolveReposWithGroup(all): %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo1", "repo2"}; !sameAliasSlice(got, want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
}

func TestResolveReposWithGroup_CustomGroupFilters(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	addRepoRecord(t, "repo2")
	addRepoRecord(t, "repo3")
	if _, err := database.CreateGroup("backend"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := database.AddRepositoriesToGroup("backend", []string{"repo3", "repo1"}); err != nil {
		t.Fatalf("AddRepositoriesToGroup: %v", err)
	}

	repos, err := resolveReposWithGroup(nil, "backend")
	if err != nil {
		t.Fatalf("resolveReposWithGroup(backend): %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo1", "repo3"}; !sameAliasSlice(got, want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
}

func TestResolveReposWithGroup_RepoAndGroupIntersectInRepoOrder(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	addRepoRecord(t, "repo2")
	addRepoRecord(t, "repo3")
	if _, err := database.CreateGroup("backend"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := database.AddRepositoriesToGroup("backend", []string{"repo1", "repo3"}); err != nil {
		t.Fatalf("AddRepositoriesToGroup: %v", err)
	}

	repos, err := resolveReposWithGroup([]string{"repo3", "repo2", "repo1"}, "backend")
	if err != nil {
		t.Fatalf("resolveReposWithGroup intersect: %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo3", "repo1"}; !sameAliasSlice(got, want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
}

func TestResolveReposWithGroup_UnknownGroupErrors(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")

	_, err := resolveReposWithGroup(nil, "missing")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("resolveReposWithGroup missing group error = %v, want ErrNotFound", err)
	}
}

func TestRunStatusWithGroupFiltersRepos(t *testing.T) {
	database = setupTestDB(t)
	repo1Dir := initRepo(t)
	if _, err := database.AddRepository("repo1", "repo1", repo1Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo1: %v", err)
	}
	repo2Dir := initRepo(t)
	if _, err := database.AddRepository("repo2", "repo2", repo2Dir, "main"); err != nil {
		t.Fatalf("AddRepository repo2: %v", err)
	}
	if _, err := database.CreateGroup("backend"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := database.AddRepositoriesToGroup("backend", []string{"repo1"}); err != nil {
		t.Fatalf("AddRepositoriesToGroup: %v", err)
	}

	if err := runStatusWithGroup(false, nil, "backend"); err != nil {
		t.Fatalf("runStatusWithGroup: %v", err)
	}
}

func addRepoRecord(t *testing.T, alias string) {
	t.Helper()
	if _, err := database.AddRepository(alias, alias, "/path/"+alias, "main"); err != nil {
		t.Fatalf("AddRepository %s: %v", alias, err)
	}
}

func repoAliases(repos []*db.Repository) []string {
	aliases := make([]string, 0, len(repos))
	for _, repo := range repos {
		aliases = append(aliases, repo.Alias)
	}
	return aliases
}

func sameAliasSlice(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

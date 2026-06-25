package cli

import (
	"errors"
	"testing"

	"github.com/alexandreafj/gitm/internal/db"
	"github.com/spf13/cobra"
)

func TestGroupCmdExists(t *testing.T) {
	cmd := groupCmd()
	if cmd == nil {
		t.Fatal("groupCmd() returned nil")
	}
}

func TestGroupCmdHasSubcommands(t *testing.T) {
	cmd := groupCmd()
	expected := []string{"list", "show", "create", "rename", "delete", "add", "remove"}
	actual := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		actual[sub.Name()] = true
	}
	for _, name := range expected {
		if !actual[name] {
			t.Errorf("group subcommand %q not found", name)
		}
	}
}

func TestGroupCmdSubcommandsRunnable(t *testing.T) {
	cmd := groupCmd()
	for _, sub := range cmd.Commands() {
		if sub.RunE == nil {
			t.Errorf("group %s has no RunE", sub.Name())
		}
	}
}

func TestRunGroupCreateAddRemoveDelete(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	addRepoRecord(t, "repo2")

	if err := runGroupCreate("backend"); err != nil {
		t.Fatalf("runGroupCreate: %v", err)
	}
	if err := runGroupAdd("backend", []string{"repo1", "repo2"}); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}
	repos, err := database.ListRepositoriesByGroup("backend")
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup: %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo1", "repo2"}; !sameAliasSlice(got, want) {
		t.Fatalf("backend repos = %v, want %v", got, want)
	}

	if err := runGroupRemove("backend", []string{"repo1"}); err != nil {
		t.Fatalf("runGroupRemove: %v", err)
	}
	repos, err = database.ListRepositoriesByGroup("backend")
	if err != nil {
		t.Fatalf("ListRepositoriesByGroup after remove: %v", err)
	}
	if got, want := repoAliases(repos), []string{"repo2"}; !sameAliasSlice(got, want) {
		t.Fatalf("backend repos after remove = %v, want %v", got, want)
	}

	if err := runGroupRename("backend", "api"); err != nil {
		t.Fatalf("runGroupRename: %v", err)
	}
	if err := runGroupDelete("api"); err != nil {
		t.Fatalf("runGroupDelete: %v", err)
	}
	if _, err := database.GetGroup("api"); !errors.Is(err, db.ErrGroupNotFound) {
		t.Fatalf("GetGroup(api) error = %v, want ErrGroupNotFound", err)
	}
}

func TestRunGroupProtectedAllErrors(t *testing.T) {
	database = setupTestDB(t)

	if err := runGroupCreate(db.DefaultGroupName); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("runGroupCreate(all) error = %v, want ErrReservedGroup", err)
	}
	if err := runGroupDelete(db.DefaultGroupName); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("runGroupDelete(all) error = %v, want ErrReservedGroup", err)
	}
	if err := runGroupAdd(db.DefaultGroupName, []string{"repo1"}); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("runGroupAdd(all) error = %v, want ErrReservedGroup", err)
	}
	if err := runGroupRemove(db.DefaultGroupName, []string{"repo1"}); !errors.Is(err, db.ErrReservedGroup) {
		t.Fatalf("runGroupRemove(all) error = %v, want ErrReservedGroup", err)
	}
}

func TestRunGroupListAndShow(t *testing.T) {
	database = setupTestDB(t)
	addRepoRecord(t, "repo1")
	if err := runGroupCreate("backend"); err != nil {
		t.Fatalf("runGroupCreate: %v", err)
	}
	if err := runGroupAdd("backend", []string{"repo1"}); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}

	if err := runGroupList(); err != nil {
		t.Fatalf("runGroupList: %v", err)
	}
	if err := runGroupShow("backend"); err != nil {
		t.Fatalf("runGroupShow: %v", err)
	}
	if err := runGroupShow(db.DefaultGroupName); err != nil {
		t.Fatalf("runGroupShow(all): %v", err)
	}
}

func TestRepoAwareCommandsHaveGroupFlag(t *testing.T) {
	commands := []*cobraCommandWithName{
		{name: "checkout", cmd: checkoutCmd()},
		{name: "status", cmd: statusCmd()},
		{name: "update", cmd: updateCmd()},
		{name: "sync", cmd: syncCmd()},
		{name: "discard", cmd: discardCmd()},
		{name: "commit", cmd: commitCmd()},
		{name: "stash", cmd: stashCmd()},
		{name: "stash apply", cmd: stashApplyCmd()},
		{name: "stash pop", cmd: stashPopCmd()},
		{name: "stash list", cmd: stashListCmd()},
		{name: "reset", cmd: resetCmd()},
		{name: "track", cmd: trackCmd()},
		{name: "untrack", cmd: untrackCmd()},
		{name: "doctor", cmd: doctorCmd()},
		{name: "branch create", cmd: branchCreateCmd()},
		{name: "branch rename", cmd: branchRenameCmd()},
		{name: "branch delete", cmd: branchDeleteCmd()},
	}
	for _, item := range commands {
		if flag := item.cmd.Flags().Lookup("group"); flag == nil {
			t.Errorf("%s missing --group flag", item.name)
		}
		if flag := item.cmd.Flags().ShorthandLookup("g"); flag == nil {
			t.Errorf("%s missing -g shorthand", item.name)
		}
	}
}

type cobraCommandWithName struct {
	name string
	cmd  *cobra.Command
}

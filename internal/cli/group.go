package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
)

func addGroupFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "group", "g", "", "Limit to repositories in a group")
}

func groupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage repository groups",
		Long: `Manage optional repository groups.

The built-in "all" group contains every registered repository and is managed
automatically. It can be listed and shown, but cannot be created, renamed,
deleted, or manually edited.`,
	}

	cmd.AddCommand(groupListCmd())
	cmd.AddCommand(groupShowCmd())
	cmd.AddCommand(groupCreateCmd())
	cmd.AddCommand(groupRenameCmd())
	cmd.AddCommand(groupDeleteCmd())
	cmd.AddCommand(groupAddCmd())
	cmd.AddCommand(groupRemoveCmd())

	return cmd
}

func groupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repository groups",
		Long:  `List all repository groups, including the built-in "all" group.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupList()
		},
	}
}

func groupShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show repositories in a group",
		Long:  `Show repositories that belong to a group. Use "all" to show every registered repository.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupShow(args[0])
		},
	}
}

func groupCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a repository group",
		Long:  `Create a custom repository group. The built-in "all" group is reserved and cannot be created manually.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupCreate(args[0])
		},
	}
}

func groupRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a repository group",
		Long:  `Rename a custom repository group. The built-in "all" group cannot be renamed.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupRename(args[0], args[1])
		},
	}
}

func groupDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a repository group",
		Long:    `Delete a custom repository group and its memberships. Repositories themselves are not removed.`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupDelete(args[0])
		},
	}
}

func groupAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <repo-alias...>",
		Short: "Add repositories to a group",
		Long:  `Add one or more registered repository aliases to a custom group. The built-in "all" group is automatic and cannot be edited manually.`,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupAdd(args[0], args[1:])
		},
	}
}

func groupRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name> <repo-alias...>",
		Short: "Remove repositories from a group",
		Long:  `Remove one or more registered repository aliases from a custom group. The built-in "all" group is automatic and cannot be edited manually.`,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGroupRemove(args[0], args[1:])
		},
	}
}

func runGroupList() error {
	groups, err := database.ListGroups()
	if err != nil {
		return fmt.Errorf("list groups: %w", err)
	}
	if len(groups) == 0 {
		fmt.Println("No groups found.")
		return nil
	}
	printGroupTable(groups)
	return nil
}

func runGroupShow(name string) error {
	group, err := database.GetGroup(name)
	if err != nil {
		return groupError("show group", name, err)
	}
	repos, err := database.ListRepositoriesByGroup(name)
	if err != nil {
		return groupError("list repositories for group", name, err)
	}

	fmt.Printf("%s\n\n", color.New(color.Bold).Sprintf("Group: %s (%d repo(s))", group.Name, group.RepoCount))
	if len(repos) == 0 {
		fmt.Println("No repositories in this group.")
		return nil
	}
	printRepoTable(repos)
	return nil
}

func runGroupCreate(name string) error {
	group, err := database.CreateGroup(name)
	if err != nil {
		return groupError("create group", name, err)
	}
	color.Green("  ✓ created group %s", group.Name)
	return nil
}

func runGroupRename(oldName, newName string) error {
	if err := database.RenameGroup(oldName, newName); err != nil {
		return groupError("rename group", oldName, err)
	}
	color.Green("  ✓ renamed group %s → %s", strings.TrimSpace(oldName), strings.TrimSpace(newName))
	return nil
}

func runGroupDelete(name string) error {
	if err := database.DeleteGroup(name); err != nil {
		return groupError("delete group", name, err)
	}
	color.Green("  ✓ deleted group %s", strings.TrimSpace(name))
	return nil
}

func runGroupAdd(name string, aliases []string) error {
	if err := database.AddRepositoriesToGroup(name, aliases); err != nil {
		return groupError("add repositories to group", name, err)
	}
	color.Green("  ✓ added %d repository(ies) to %s", len(uniqueStrings(aliases)), strings.TrimSpace(name))
	return nil
}

func runGroupRemove(name string, aliases []string) error {
	if err := database.RemoveRepositoriesFromGroup(name, aliases); err != nil {
		return groupError("remove repositories from group", name, err)
	}
	color.Green("  ✓ removed %d repository(ies) from %s", len(uniqueStrings(aliases)), strings.TrimSpace(name))
	return nil
}

func groupError(action, name string, err error) error {
	groupName := strings.TrimSpace(name)
	switch {
	case errors.Is(err, db.ErrReservedGroup):
		return fmt.Errorf("%s %q: %w", action, groupName, err)
	case errors.Is(err, db.ErrInvalidGroupName):
		return fmt.Errorf("%s %q: group names cannot be empty, contain spaces, or contain commas: %w", action, groupName, err)
	case errors.Is(err, db.ErrNotFound):
		return fmt.Errorf("%s %q: not found: %w", action, groupName, err)
	default:
		return fmt.Errorf("%s %q: %w", action, groupName, err)
	}
}

func printGroupTable(groups []*db.Group) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgWhite)

	fmt.Printf("%-24s  %-10s  %s\n",
		header.Sprint("GROUP"),
		header.Sprint("REPOS"),
		header.Sprint("TYPE"),
	)
	for _, group := range groups {
		kind := "custom"
		if group.Name == db.DefaultGroupName {
			kind = "built-in"
		}
		fmt.Printf("%-24s  %-10d  %s\n",
			cyan.Sprint(group.Name),
			group.RepoCount,
			dim.Sprint(kind),
		)
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

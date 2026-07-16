package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/db"
)

func contextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Manage repository contexts",
		Long: `Manage repository contexts.

A context is an isolated working set of repositories. Every repository belongs
to exactly one context, and all multi-repo commands (checkout, branch, status,
commit, push, stash, reset, sync, ...) operate only on the repositories of the
currently active context.

The built-in "default" context always exists: new installations start on it,
and repositories live there until moved to a custom context. It cannot be
renamed or deleted.

Switch contexts with "gitm context use <name>" — the choice is persisted, so
every following gitm invocation stays scoped to that context until you switch
again.`,
		Example: `  # Create a context per client and move repos into it
  gitm context create company-a
  gitm context add company-a api-gateway auth-service

  # Switch to it: from now on all commands only touch company-a repos
  gitm context use company-a
  gitm checkout feature/JIRA-123
  gitm branch create feature/JIRA-456

  # Jump back to the default context
  gitm context use default`,
	}

	cmd.AddCommand(contextListCmd())
	cmd.AddCommand(contextShowCmd())
	cmd.AddCommand(contextCurrentCmd())
	cmd.AddCommand(contextCreateCmd())
	cmd.AddCommand(contextUseCmd())
	cmd.AddCommand(contextRenameCmd())
	cmd.AddCommand(contextDeleteCmd())
	cmd.AddCommand(contextAddCmd())
	cmd.AddCommand(contextRemoveCmd())

	return cmd
}

func contextListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repository contexts",
		Long:  `List all repository contexts. The active context is marked with an asterisk.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextList()
		},
	}
}

func contextShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show repositories in a context",
		Long:  `Show repositories that belong to a context. With no argument, shows the active context.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runContextShow(name)
		},
	}
}

func contextCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the active context",
		Long:  `Print the name of the currently active context.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextCurrent()
		},
	}
}

func contextCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a repository context",
		Long:  `Create a custom repository context. The built-in "default" context is reserved and cannot be created manually.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextCreate(args[0])
		},
	}
}

func contextUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <name>",
		Aliases: []string{"switch"},
		Short:   "Switch the active context",
		Long: `Switch the active context. The choice is persisted: every following gitm
invocation operates only on the repositories of that context until you switch again.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextUse(args[0])
		},
	}
}

func contextRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a repository context",
		Long:  `Rename a custom repository context. The built-in "default" context cannot be renamed.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextRename(args[0], args[1])
		},
	}
}

func contextDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a repository context",
		Long: `Delete a custom repository context. Its repositories are moved back to the
built-in "default" context — they are never unregistered. The active context
and the "default" context cannot be deleted.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextDelete(args[0])
		},
	}
}

func contextAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "add <name> <repo-alias...>",
		Aliases: []string{"assign"},
		Short:   "Move repositories into a context",
		Long: `Move one or more registered repositories into a context.

A repository belongs to exactly one context, so adding it to a context removes
it from the one it was in before.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextAdd(args[0], args[1:])
		},
	}
}

func contextRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name> <repo-alias...>",
		Short: "Move repositories out of a context back to default",
		Long: `Move one or more repositories out of a context and back into the built-in
"default" context. The repositories stay registered.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextRemove(args[0], args[1:])
		},
	}
}

func runContextList() error {
	contexts, err := database.ListContexts()
	if err != nil {
		return fmt.Errorf("list contexts: %w", err)
	}
	printContextTable(contexts)
	return nil
}

func runContextShow(name string) error {
	var context *db.Context
	var err error
	if strings.TrimSpace(name) == "" {
		context, err = database.ActiveContext()
	} else {
		context, err = database.GetContext(name)
	}
	if err != nil {
		return contextError("show context", name, err)
	}

	repos, err := database.ListRepositoriesByContext(context.Name)
	if err != nil {
		return contextError("list repositories for context", context.Name, err)
	}

	marker := ""
	if context.Active {
		marker = " (active)"
	}
	fmt.Printf("%s\n\n", color.New(color.Bold).Sprintf("Context: %s%s (%d repo(s))", context.Name, marker, context.RepoCount))
	if len(repos) == 0 {
		fmt.Println("No repositories in this context.")
		return nil
	}
	printRepoTable(repos)
	return nil
}

func runContextCurrent() error {
	context, err := database.ActiveContext()
	if err != nil {
		return fmt.Errorf("read active context: %w", err)
	}
	fmt.Println(context.Name)
	return nil
}

func runContextCreate(name string) error {
	context, err := database.CreateContext(name)
	if err != nil {
		return contextError("create context", name, err)
	}
	color.Green("  ✓ created context %s", context.Name)
	fmt.Printf("    Switch to it with `gitm context use %s`.\n", context.Name)
	return nil
}

func runContextUse(name string) error {
	context, err := database.SetActiveContext(name)
	if err != nil {
		return contextError("switch to context", name, err)
	}
	color.Green("  ✓ switched to context %s (%d repo(s))", context.Name, context.RepoCount)
	return nil
}

func runContextRename(oldName, newName string) error {
	if err := database.RenameContext(oldName, newName); err != nil {
		return contextError("rename context", oldName, err)
	}
	color.Green("  ✓ renamed context %s → %s", strings.TrimSpace(oldName), strings.TrimSpace(newName))
	return nil
}

func runContextDelete(name string) error {
	moved, err := database.DeleteContext(name)
	if err != nil {
		return contextError("delete context", name, err)
	}
	color.Green("  ✓ deleted context %s", strings.TrimSpace(name))
	if moved > 0 {
		fmt.Printf("    %d repository(ies) moved back to the %q context.\n", moved, db.DefaultContextName)
	}
	return nil
}

func runContextAdd(name string, aliases []string) error {
	if err := database.AssignRepositoriesToContext(name, aliases); err != nil {
		return contextError("add repositories to context", name, err)
	}
	color.Green("  ✓ moved %d repository(ies) into %s", len(uniqueStrings(aliases)), strings.TrimSpace(name))
	return nil
}

func runContextRemove(name string, aliases []string) error {
	context, err := database.GetContext(name)
	if err != nil {
		return contextError("find context", name, err)
	}
	// Only move repos that are actually in this context, so `remove` cannot
	// silently yank repos out of an unrelated context.
	inContext, err := database.ListRepositoriesByContext(context.Name)
	if err != nil {
		return contextError("list repositories for context", name, err)
	}
	members := make(map[string]bool, len(inContext))
	for _, repo := range inContext {
		members[repo.Alias] = true
	}
	for _, alias := range uniqueStrings(aliases) {
		if !members[alias] {
			return fmt.Errorf("repository %q is not in context %q — run `gitm context show %s` to see its repos", alias, context.Name, context.Name)
		}
	}

	if err := database.AssignRepositoriesToContext(db.DefaultContextName, aliases); err != nil {
		return contextError("remove repositories from context", name, err)
	}
	color.Green("  ✓ moved %d repository(ies) back to %s", len(uniqueStrings(aliases)), db.DefaultContextName)
	return nil
}

func contextError(action, name string, err error) error {
	contextName := strings.TrimSpace(name)
	switch {
	case errors.Is(err, db.ErrContextNotFound):
		return fmt.Errorf("%s %q: context not found — run `gitm context list` to see contexts: %w", action, contextName, err)
	case errors.Is(err, db.ErrInvalidContextName):
		return fmt.Errorf("%s %q: context names cannot be empty, contain spaces, or contain commas: %w", action, contextName, err)
	case errors.Is(err, db.ErrContextActive):
		return fmt.Errorf("%s %q: cannot delete the active context — switch first with `gitm context use %s`: %w", action, contextName, db.DefaultContextName, err)
	default:
		return fmt.Errorf("%s %q: %w", action, contextName, err)
	}
}

func printContextTable(contexts []*db.Context) {
	header := color.New(color.Bold, color.Underline)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgWhite)

	fmt.Printf("%-2s %-24s  %-10s  %s\n",
		"",
		header.Sprint("CONTEXT"),
		header.Sprint("REPOS"),
		header.Sprint("TYPE"),
	)
	for _, context := range contexts {
		marker := " "
		name := cyan.Sprint(context.Name)
		if context.Active {
			marker = color.GreenString("*")
			name = color.New(color.FgGreen, color.Bold).Sprint(context.Name)
		}
		kind := "custom"
		if context.Name == db.DefaultContextName {
			kind = "built-in"
		}
		fmt.Printf("%-2s %-24s  %-10d  %s\n",
			marker,
			name,
			context.RepoCount,
			dim.Sprint(kind),
		)
	}
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreferreira/gitm/internal/config"
	"github.com/alexandreferreira/gitm/internal/db"
)

var (
	cfg      *config.Config
	database *db.DB
)

// Root returns the root cobra command with all sub-commands attached.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "gitm",
		Short: "Multi-repository git manager",
		Long: `gitm — Manage git operations across multiple repositories in parallel.

Registered repositories are stored in ~/.gitm/gitm.db.
All multi-repo operations run concurrently for speed.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip DB init for completion commands.
			if cmd.Name() == "__complete" || cmd.Name() == "help" {
				return nil
			}

			var err error
			cfg, err = config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			database, err = db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if database != nil {
				_ = database.Close()
			}
		},
	}

	// Sub-command groups.
	root.AddCommand(repoCmd())
	root.AddCommand(checkoutCmd())
	root.AddCommand(branchCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(updateCmd())
	root.AddCommand(discardCmd())
	root.AddCommand(commitCmd())
	root.AddCommand(stashCmd())
	root.AddCommand(resetCmd())

	return root
}

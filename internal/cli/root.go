package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alexandreafj/gitm/internal/config"
	"github.com/alexandreafj/gitm/internal/db"
)

var (
	cfg      *config.Config
	database *db.DB
)

// Root returns the root cobra command with all sub-commands attached.
func Root(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "gitm",
		Version: version,
		Short:   "Multi-repository git manager",
		Long: `gitm — Manage git operations across multiple repositories in parallel.

Registered repositories are stored in ~/.gitm/gitm.db.
All multi-repo operations run concurrently for speed.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "__complete" || cmd.Name() == "help" || cmd.Name() == "upgrade" {
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
	root.AddCommand(trackCmd())
	root.AddCommand(untrackCmd())
	root.AddCommand(upgradeCmd(version))

	return root
}

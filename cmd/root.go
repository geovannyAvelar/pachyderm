package cmd

import (
	"os"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pachyderm",
	Short: "A version manager for PostgreSQL",
	Long: `Pachyderm installs, switches between, and runs multiple PostgreSQL
versions locally, with no root or administrator rights.

  pachyderm get 17      install PostgreSQL 17 and make it the active version
  pachyderm start       start its server (initializing it on first use)
  pachyderm psql        connect to it
  pachyderm local 16    pin the current directory to PostgreSQL 16`,
	// Usage is for mistakes in how a command is called; printing it after
	// every runtime error (a failed download, a stopped server) is noise.
	SilenceUsage: true,
	// Older releases kept one data directory per release (data/16.14.0);
	// move those to one per major version (data/16) before any command
	// looks at them. A failure here is reported but doesn't block the
	// command: the operations that create data directories check again.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		notices, err := postgres.MigrateLegacyDataDirs()
		for _, notice := range notices {
			cmd.PrintErrln(notice)
		}
		if err != nil {
			cmd.PrintErrf("Could not migrate data directories: %v\n", err)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

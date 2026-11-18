package cmd

import (
	"os"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pachyderm",
	Short: "PostgreSQL management tool",
	Long:  `PostgreSQL management tool`,
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

func init() {

}

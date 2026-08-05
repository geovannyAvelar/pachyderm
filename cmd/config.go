package cmd

import (
	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config <version>",
	Short: "Show where a PostgreSQL version's config files live",
	Long:  `Print the data directory and the paths to postgresql.conf, pg_hba.conf, and pg_ident.conf for an installed, initialized PostgreSQL version.`,
	Args:  cobra.ExactArgs(1),
	RunE:  showConfigFiles,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func showConfigFiles(cmd *cobra.Command, args []string) error {
	version, err := resolveInstalledVersion(args[0])
	if err != nil {
		return err
	}

	files, err := postgres.GetConfigFiles(version)
	if err != nil {
		return err
	}

	cmd.Printf("Data directory:   %s\n", files.DataDir)
	cmd.Printf("postgresql.conf:  %s\n", files.ConfigFile)
	cmd.Printf("pg_hba.conf:      %s\n", files.HBAFile)
	cmd.Printf("pg_ident.conf:    %s\n", files.IdentFile)

	return nil
}

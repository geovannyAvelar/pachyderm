package cmd

import (
	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <version>",
	Short: "Uninstall a PostgreSQL version",
	Long:  `Remove a locally installed PostgreSQL version.`,
	Args:  cobra.ExactArgs(1),
	RunE:  uninstallPostgres,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func uninstallPostgres(cmd *cobra.Command, args []string) error {
	version, err := resolveInstalledVersion(args[0])
	if err != nil {
		return err
	}

	if err := postgres.Uninstall(version); err != nil {
		return err
	}

	cmd.Printf("Uninstalled PostgreSQL %s.\n", version)
	return nil
}

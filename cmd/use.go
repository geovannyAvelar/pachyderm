package cmd

import (
	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <version>",
	Short: "Switch the active PostgreSQL version",
	Long:  `Point the "current" PostgreSQL version at an already-installed version.`,
	Args:  cobra.ExactArgs(1),
	RunE:  usePostgres,
}

func init() {
	rootCmd.AddCommand(useCmd)
}

func usePostgres(cmd *cobra.Command, args []string) error {
	version, err := resolveInstalledVersion(args[0])
	if err != nil {
		return err
	}

	if err := postgres.SetCurrent(version); err != nil {
		return err
	}

	cmd.Printf("Set PostgreSQL %s as the active version.\n", version)
	warnIfBinDirMissingFromPath(cmd)
	return nil
}

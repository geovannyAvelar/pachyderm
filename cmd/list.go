package cmd

import (
	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List PostgreSQL versions",
	Long:  `List PostgreSQL versions`,
	Run:   listPostgresVersions,
}

func init() {
	listCmd.Flags().StringP("file", "f", "versions.json", "JSON file containing versions")
	rootCmd.AddCommand(listCmd)
}

func listPostgresVersions(cmd *cobra.Command, args []string) {
	versions, err := postgres.GetVersions("versions.json")
	if err != nil {
		cmd.Fatalf("failed to get versions: %v", err)
	}

}

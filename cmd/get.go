package cmd

import (
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Download and install PostgreSQL",
	Long:  `Download and install PostgreSQL`,
	Args:  cobra.MinimumNArgs(1),
	Run:   getPostgres,
}

func init() {
	rootCmd.AddCommand(getCmd)
}

func getPostgres(cmd *cobra.Command, args []string) {
	version := args[0]
	cmd.Printf("Selected PostgreSQL version: %s\n", version)
}

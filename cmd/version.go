package cmd

import "github.com/spf13/cobra"

// version is set at build time via -ldflags "-X pachyderm/cmd.version=...".
// It defaults to "dev" for local, non-release builds.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the pachyderm version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println(version)
		return nil
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(versionCmd)
}

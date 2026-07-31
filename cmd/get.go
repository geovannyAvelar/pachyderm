package cmd

import (
	"fmt"
	"os"
	"strings"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <version>",
	Short: "Download and install PostgreSQL",
	Long:  `Download and install a PostgreSQL version and set it as the active version.`,
	Args:  cobra.ExactArgs(1),
	RunE:  getPostgres,
}

func init() {
	rootCmd.AddCommand(getCmd)
}

func getPostgres(cmd *cobra.Command, args []string) error {
	version := args[0]

	target, err := postgres.Target()
	if err != nil {
		return err
	}

	cmd.Printf("Looking up PostgreSQL %s releases...\n", version)
	tags, err := postgres.FetchReleaseTags()
	if err != nil {
		return fmt.Errorf("failed to look up available releases: %w", err)
	}

	tag, err := postgres.SelectTag(tags, version)
	if err != nil {
		return err
	}

	installed, err := postgres.IsInstalled(tag)
	if err != nil {
		return err
	}

	if installed {
		cmd.Printf("PostgreSQL %s is already installed.\n", tag)
	} else {
		cmd.Printf("Downloading PostgreSQL %s (%s)...\n", tag, target)
		if err := postgres.Install(tag, target); err != nil {
			return fmt.Errorf("failed to install PostgreSQL %s: %w", tag, err)
		}
		cmd.Printf("Installed PostgreSQL %s.\n", tag)
	}

	if err := postgres.SetCurrent(tag); err != nil {
		return err
	}
	cmd.Printf("Set PostgreSQL %s as the active version.\n", tag)

	warnIfBinDirMissingFromPath(cmd)
	return nil
}

func warnIfBinDirMissingFromPath(cmd *cobra.Command) {
	binDir, err := postgres.CurrentBinDir()
	if err != nil {
		return
	}

	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if p == binDir {
			return
		}
	}

	cmd.Printf("\nAdd this to your shell profile to use it by default:\n  export PATH=\"%s:$PATH\"\n", binDir)
}

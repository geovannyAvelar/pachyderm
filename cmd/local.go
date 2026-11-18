package cmd

import (
	"os"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local <version>",
	Short: "Pin this directory to a PostgreSQL version",
	Long: `Write a .pachyderm-version file in the current directory. In this
directory and every directory under it, pachyderm commands given no version
(start, stop, psql, exec, get, ...) use this one. It may be a major (16), a
minor (16.10), or an exact release (16.14.0); a major or minor means the
newest installed release of it.

Pachyderm also reads the "postgres" line of an asdf .tool-versions file,
and $PACHYDERM_VERSION overrides both.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		path, err := postgres.WriteProjectVersion(wd, args[0])
		if err != nil {
			return err
		}
		cmd.Printf("Wrote %s (PostgreSQL %s).\n", path, args[0])

		if _, ok, err := postgres.LatestInstalledMatch(args[0]); err == nil && !ok {
			cmd.Printf("PostgreSQL %s is not installed yet; run \"pachyderm get\" to install it.\n", args[0])
		}
		return nil
	},
}

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show which PostgreSQL version commands here use, and why",
	Long:  `Print the installed PostgreSQL release that pachyderm commands given no version use in the current directory, and where that choice came from.` + versionArgHelp,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		version, source, err := resolveTargetVersion(nil)
		if err != nil {
			return err
		}
		switch source.From {
		case "current":
			cmd.Printf("%s (the active version; set with \"pachyderm use\")\n", version)
		default:
			cmd.Printf("%s (PostgreSQL %s, from %s)\n", version, source.Requested, source.From)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(localCmd, currentCmd)
}

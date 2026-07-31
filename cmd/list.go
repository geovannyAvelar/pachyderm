package cmd

import (
	"text/tabwriter"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List PostgreSQL versions",
	Long:  `List available PostgreSQL major versions, or the versions installed locally.`,
	RunE:  listPostgresVersions,
}

func init() {
	listCmd.Flags().StringP("file", "f", "", "JSON file or URL to load the version catalog from (default: embedded catalog)")
	listCmd.Flags().Bool("installed", false, "List locally installed versions instead of available ones")
	rootCmd.AddCommand(listCmd)
}

func listPostgresVersions(cmd *cobra.Command, args []string) error {
	installedOnly, err := cmd.Flags().GetBool("installed")
	if err != nil {
		return err
	}

	if installedOnly {
		return listInstalledVersions(cmd)
	}

	file, err := cmd.Flags().GetString("file")
	if err != nil {
		return err
	}

	versions, err := postgres.GetVersions(file)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	defer w.Flush()

	w.Write([]byte("VERSION\tCURRENT MINOR\tSUPPORTED\n"))
	for _, v := range versions {
		supported := "no"
		if v.Supported {
			supported = "yes"
		}
		w.Write([]byte(v.Version + "\t" + v.CurrentMinor + "\t" + supported + "\n"))
	}

	return nil
}

func listInstalledVersions(cmd *cobra.Command) error {
	versions, err := postgres.ListInstalled()
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		cmd.Println("No PostgreSQL versions installed. Run \"pachyderm get <version>\" to install one.")
		return nil
	}

	current, err := postgres.CurrentVersion()
	if err != nil {
		return err
	}

	for _, v := range versions {
		if v == current {
			cmd.Printf("* %s (current)\n", v)
		} else {
			cmd.Printf("  %s\n", v)
		}
	}

	return nil
}

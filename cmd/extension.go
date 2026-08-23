package cmd

import (
	"text/tabwriter"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var extensionCmd = &cobra.Command{
	Use:   "extension",
	Short: "Manage PostgreSQL extensions",
	Long:  `List, install, and uninstall extensions on a running PostgreSQL server.`,
}

var extensionListCmd = &cobra.Command{
	Use:   "list <version>",
	Short: "List extensions available to a running server",
	Long:  `List every extension available to a running version's server, and whether it's currently installed.`,
	Args:  cobra.ExactArgs(1),
	RunE:  listExtensions,
}

var extensionInstallCmd = &cobra.Command{
	Use:   "install <version> <name>",
	Short: "Enable an extension",
	Long:  `Enable an extension (CREATE EXTENSION) on a running version's server.`,
	Args:  cobra.ExactArgs(2),
	RunE:  installExtension,
}

var extensionUninstallCmd = &cobra.Command{
	Use:   "uninstall <version> <name>",
	Short: "Disable an extension",
	Long:  `Disable an extension (DROP EXTENSION) on a running version's server.`,
	Args:  cobra.ExactArgs(2),
	RunE:  uninstallExtension,
}

var extensionGetPostGISCmd = &cobra.Command{
	Use:   "get-postgis <version> [postgis-version]",
	Short: "Download prebuilt PostGIS binaries",
	Long: `Download a prebuilt PostGIS binary from geovannyAvelar/postgis-binaries and
overlay it onto an installed PostgreSQL version, making it available to
"extension install". postgis-version defaults to the latest published
release in the ` + postgres.DefaultPostGISVersion + ` series.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: getPostGIS,
}

func init() {
	extensionCmd.AddCommand(extensionListCmd, extensionInstallCmd, extensionUninstallCmd, extensionGetPostGISCmd)
	rootCmd.AddCommand(extensionCmd)
}

func listExtensions(cmd *cobra.Command, args []string) error {
	version, port, err := resolveRunningVersion(args[0])
	if err != nil {
		return err
	}

	extensions, err := postgres.ListExtensions(version, port)
	if err != nil {
		return err
	}

	if len(extensions) == 0 {
		cmd.Println("No extensions available.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	defer w.Flush()

	w.Write([]byte("NAME\tVERSION\tINSTALLED\tCOMMENT\n"))
	for _, e := range extensions {
		installed := "no"
		if e.Installed {
			installed = "yes"
		}
		w.Write([]byte(e.Name + "\t" + e.Version + "\t" + installed + "\t" + e.Comment + "\n"))
	}

	return nil
}

func installExtension(cmd *cobra.Command, args []string) error {
	version, port, err := resolveRunningVersion(args[0])
	if err != nil {
		return err
	}
	name := args[1]

	if err := postgres.InstallExtension(version, port, name); err != nil {
		return err
	}

	cmd.Printf("Installed extension %s on PostgreSQL %s.\n", name, version)
	return nil
}

func uninstallExtension(cmd *cobra.Command, args []string) error {
	version, port, err := resolveRunningVersion(args[0])
	if err != nil {
		return err
	}
	name := args[1]

	if err := postgres.UninstallExtension(version, port, name); err != nil {
		return err
	}

	cmd.Printf("Uninstalled extension %s from PostgreSQL %s.\n", name, version)
	return nil
}

func getPostGIS(cmd *cobra.Command, args []string) error {
	version, err := resolveInstalledVersion(args[0])
	if err != nil {
		return err
	}

	postgisVersion := postgres.DefaultPostGISVersion
	if len(args) == 2 {
		postgisVersion = args[1]
	}

	cmd.Printf("Looking up PostGIS %s releases...\n", postgisVersion)
	if err := postgres.InstallPostGISBinary(version, postgisVersion); err != nil {
		return err
	}

	cmd.Printf("Installed PostGIS %s for PostgreSQL %s.\n", postgisVersion, version)
	return nil
}

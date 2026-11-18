package cmd

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

// versionArgHelp is appended to the help of every command that takes an
// optional version.
const versionArgHelp = `

With no version, it uses the version in $PACHYDERM_VERSION, else the one in
the nearest .pachyderm-version or .tool-versions file, else the active
("current") version. A major or minor version (e.g. 16) means the newest
installed release of it.`

var initCmd = &cobra.Command{
	Use:   "init [version]",
	Short: "Initialize a PostgreSQL version's data directory",
	Long: `Create the data directory for a PostgreSQL version with initdb, if it
doesn't exist yet. Every release of a major version shares one data
directory.` + versionArgHelp,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var startPort int

var startCmd = &cobra.Command{
	Use:   "start [version]",
	Short: "Start a PostgreSQL server",
	Long: `Start the server of a PostgreSQL version in the background, on its
configured port (5432 unless changed). The data directory is initialized
first if needed. Starting a server that's already running does nothing.` + versionArgHelp,
	Args: cobra.MaximumNArgs(1),
	RunE: runStart,
}

var stopCmd = &cobra.Command{
	Use:   "stop [version]",
	Short: "Stop a PostgreSQL server",
	Long: `Stop the running server of a PostgreSQL version. Stopping a server that
isn't running does nothing.` + versionArgHelp,
	Args: cobra.MaximumNArgs(1),
	RunE: runStop,
}

var restartCmd = &cobra.Command{
	Use:   "restart [version]",
	Short: "Restart a PostgreSQL server",
	Long: `Stop the server of a PostgreSQL version if it's running, then start it
again, e.g. to apply a change to postgresql.conf or to move to a newer
release of the same major version.` + versionArgHelp,
	Args: cobra.MaximumNArgs(1),
	RunE: runRestart,
}

var statusCmd = &cobra.Command{
	Use:   "status [version]",
	Short: "Show which PostgreSQL servers are running",
	Long: `Show every installed PostgreSQL version and whether its server is
running, with its PID and port. Given a version, show only that one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func init() {
	startCmd.Flags().IntVarP(&startPort, "port", "p", 0, "port to start on, saved for later starts of this major version")
	restartCmd.Flags().IntVarP(&startPort, "port", "p", 0, "port to start on, saved for later starts of this major version")
	rootCmd.AddCommand(initCmd, startCmd, stopCmd, restartCmd, statusCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	version, _, err := resolveTargetVersion(args)
	if err != nil {
		return err
	}
	return initDataDir(cmd, version)
}

// initDataDir initializes version's data directory if needed, saying so.
func initDataDir(cmd *cobra.Command, version string) error {
	initialized, err := postgres.IsDataDirInitialized(version)
	if err != nil {
		return err
	}
	dir, err := postgres.DataDir(version)
	if err != nil {
		return err
	}
	if initialized {
		cmd.Printf("The PostgreSQL %s data directory is already initialized at %s.\n", postgres.MajorVersion(version), dir)
		return nil
	}

	cmd.Printf("Initializing the PostgreSQL %s data directory at %s...\n", postgres.MajorVersion(version), dir)
	if err := postgres.InitDB(version); err != nil {
		return err
	}
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	version, _, err := resolveTargetVersion(args)
	if err != nil {
		return err
	}
	return startServer(cmd, version)
}

func startServer(cmd *cobra.Command, version string) error {
	if startPort != 0 {
		if err := postgres.SetPort(version, startPort); err != nil {
			return err
		}
	}

	if running, _, err := postgres.ServerStatus(version); err != nil {
		return err
	} else if running {
		port, err := postgres.RunningPort(version)
		if err != nil {
			return err
		}
		if startPort != 0 && port != startPort {
			cmd.Printf("PostgreSQL %s is already running on port %d. Restart it to move it to port %d.\n", version, port, startPort)
			return nil
		}
		cmd.Printf("PostgreSQL %s is already running on port %d.\n", version, port)
		return nil
	}

	if initialized, err := postgres.IsDataDirInitialized(version); err != nil {
		return err
	} else if !initialized {
		if err := initDataDir(cmd, version); err != nil {
			return err
		}
	}

	port, err := postgres.GetPort(version)
	if err != nil {
		return err
	}
	if err := postgres.StartServer(version, port); err != nil {
		return err
	}

	cmd.Printf("Started PostgreSQL %s on port %d. Connect with: pachyderm psql\n", version, port)
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	version, _, err := resolveTargetVersion(args)
	if err != nil {
		return err
	}
	_, err = stopServer(cmd, version)
	return err
}

// stopServer stops version's server if it's running, and reports whether
// it was.
func stopServer(cmd *cobra.Command, version string) (bool, error) {
	running, _, err := postgres.ServerStatus(version)
	if err != nil {
		return false, err
	}
	if !running {
		cmd.Printf("PostgreSQL %s is not running.\n", version)
		return false, nil
	}

	if err := postgres.StopServer(version); err != nil {
		return false, err
	}
	cmd.Printf("Stopped PostgreSQL %s.\n", version)
	return true, nil
}

func runRestart(cmd *cobra.Command, args []string) error {
	version, _, err := resolveTargetVersion(args)
	if err != nil {
		return err
	}
	if _, err := stopServer(cmd, version); err != nil {
		return err
	}
	return startServer(cmd, version)
}

func runStatus(cmd *cobra.Command, args []string) error {
	var versions []string
	if len(args) > 0 {
		version, _, err := resolveTargetVersion(args)
		if err != nil {
			return err
		}
		versions = []string{version}
	} else {
		installed, err := postgres.ListInstalled()
		if err != nil {
			return err
		}
		if len(installed) == 0 {
			cmd.Println("No PostgreSQL versions installed. Run \"pachyderm get <version>\" to install one.")
			return nil
		}
		versions = installed
	}

	current, err := postgres.CurrentVersion()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "VERSION\tSTATUS\tPID\tPORT")
	for _, v := range versions {
		status, pid, port, err := versionStatus(v)
		if err != nil {
			return err
		}
		name := v
		if v == current {
			name += " (current)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, status, pid, port)
	}
	return nil
}

// versionStatus returns the STATUS, PID, and PORT columns of "status" for
// one version. A stopped server's port is the one it will start on.
func versionStatus(version string) (status, pid, port string, err error) {
	initialized, err := postgres.IsDataDirInitialized(version)
	if err != nil {
		return "", "", "", err
	}
	if !initialized {
		return "not initialized", "-", "-", nil
	}

	running, p, err := postgres.ServerStatus(version)
	if err != nil {
		return "", "", "", err
	}
	if running {
		listening, err := postgres.RunningPort(version)
		if err != nil {
			return "", "", "", err
		}
		return "running", strconv.Itoa(p), strconv.Itoa(listening), nil
	}

	configured, err := postgres.GetPort(version)
	if err != nil {
		return "", "", "", err
	}
	return "stopped", "-", strconv.Itoa(configured), nil
}

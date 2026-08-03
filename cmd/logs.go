package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var (
	logsLines  int
	logsFollow bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <version>",
	Short: "Show a PostgreSQL instance's server log",
	Long:  `Print the server log for an installed PostgreSQL version, written by pg_ctl each time the server starts.`,
	Args:  cobra.ExactArgs(1),
	RunE:  showLogs,
}

func init() {
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "number of lines to show")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow the log as it grows")
	rootCmd.AddCommand(logsCmd)
}

func showLogs(cmd *cobra.Command, args []string) error {
	version, err := resolveInstalledVersion(args[0])
	if err != nil {
		return err
	}

	tail, err := postgres.TailLog(version, logsLines)
	if err != nil {
		return err
	}
	if tail != "" {
		cmd.Println(tail)
	}

	if !logsFollow {
		return nil
	}

	logFile, err := postgres.LogFile(version)
	if err != nil {
		return err
	}
	return followLog(logFile, cmd.OutOrStdout())
}

// followLog streams lines appended to path, like `tail -f`, until the
// process is interrupted.
func followLog(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Fprint(w, line)
		}
		if err != nil {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

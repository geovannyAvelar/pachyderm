package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"pachyderm/postgres"

	"github.com/spf13/cobra"
)

var psqlCmd = &cobra.Command{
	Use:   "psql [version] [psql-args...]",
	Short: "Open psql connected to a running PostgreSQL server",
	Long: `Run psql from a PostgreSQL version's own binaries, connected to that
version's running server, on the "postgres" database unless you name
another. Every argument after the optional version goes to psql as it is:

  pachyderm psql
  pachyderm psql -c 'SELECT version()'
  pachyderm psql 15 -d mydb` + versionArgHelp,
	// Every flag belongs to psql (-h, -c, -d, ...), so pachyderm parses none.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		version, rest := splitVersionArg(args)
		return runTool(version, "psql", rest, true)
	},
}

var execCmd = &cobra.Command{
	Use:   "exec [version] <program> [args...]",
	Short: "Run a PostgreSQL program with the right version and connection",
	Long: `Run one of a PostgreSQL version's programs (pg_dump, pg_restore, createdb,
pgbench, ...) with the arguments given. If that version's server is
running, PGHOST and PGPORT point at it, and PGDATABASE is "postgres" unless
already set:

  pachyderm exec createdb myapp
  pachyderm exec pg_dump -Fc myapp -f myapp.dump
  pachyderm exec 15 pg_dump --version` + versionArgHelp,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		version, rest := splitVersionArg(args)
		if len(rest) == 0 {
			return fmt.Errorf("missing the program to run; see \"pachyderm help exec\"")
		}
		return runTool(version, rest[0], rest[1:], false)
	},
}

func init() {
	rootCmd.AddCommand(psqlCmd, execCmd)
}

// splitVersionArg splits a leading PostgreSQL version ("16", "16.10",
// "16.14.0") off args. No program or psql argument looks like one. A
// leading "--" is dropped, for people used to separating arguments that way.
func splitVersionArg(args []string) (version []string, rest []string) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 && postgres.IsVersionSpec(args[0]) {
		return args[:1], args[1:]
	}
	return nil, args
}

// runTool runs program from the bin directory of the version
// resolveTargetVersion picks from versionArgs, with args,
// connected to that version's server. With needServer, the server must be
// running.
func runTool(versionArgs []string, program string, args []string, needServer bool) error {
	if strings.ContainsAny(program, `/\`) {
		return fmt.Errorf("%q is not a PostgreSQL program name", program)
	}

	version, _, err := resolveTargetVersion(versionArgs)
	if err != nil {
		return err
	}

	dir, err := postgres.InstallDir(version)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "bin", program)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("PostgreSQL %s has no program named %s", version, program)
	}

	running, _, err := postgres.ServerStatus(version)
	if err != nil {
		return err
	}
	var port int
	if running {
		if port, err = postgres.RunningPort(version); err != nil {
			return err
		}
	} else if needServer {
		return fmt.Errorf("PostgreSQL %s is not running; start it with \"pachyderm start %s\"", version, version)
	}

	return runForeground(path, args, toolEnv(os.Environ(), port))
}

// toolEnv returns env with the libpq variables that connect a PostgreSQL
// program to the server on port over TCP loopback. PGHOST and PGPORT always
// point at that server, since that is the point of running through
// pachyderm; PGDATABASE defaults to "postgres", which initdb always creates
// (the default, a database named after the OS user, doesn't exist). With
// port 0 (no server running), env is returned unchanged.
func toolEnv(env []string, port int) []string {
	if port == 0 {
		return env
	}

	out := make([]string, 0, len(env)+3)
	hasDatabase := false
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "PGHOST", "PGPORT", "PGHOSTADDR":
			continue
		case "PGDATABASE":
			hasDatabase = true
		}
		out = append(out, kv)
	}
	out = append(out, "PGHOST=127.0.0.1", "PGPORT="+strconv.Itoa(port))
	if !hasDatabase {
		out = append(out, "PGDATABASE=postgres")
	}
	return out
}

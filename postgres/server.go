package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DataDir returns the data directory for a version's PostgreSQL cluster.
func DataDir(version string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "data", version), nil
}

// maxSocketPathLen is a conservative bound on a Unix-domain socket path; see
// fitsSocketPath.
const maxSocketPathLen = 90

// SocketDir returns the directory a version's server listens for Unix-domain
// socket connections in. Unlike the Debian-style /var/run/postgresql, this
// lives entirely under the user's home directory, so no root/admin rights
// are ever needed to create or write to it.
func SocketDir(version string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "run", version), nil
}

// socketConnectDir returns the directory to hand postgres's -k flag (or a
// client's -h) for version's Unix-domain socket at port, creating a short
// symlink under the OS temp directory if the real SocketDir path is too
// long for the kernel's socket path limit. bind()/connect() check the
// literal path they're given, not where a symlink resolves to, so this
// works around the limit without moving the socket file itself -- a deeply
// nested $HOME no longer has to fall back to TCP-only. ok is false only if
// even that symlink path doesn't fit, which would need an unusually deep OS
// temp directory.
func socketConnectDir(version string, port int) (dir string, ok bool, err error) {
	realDir, err := SocketDir(version)
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		return "", false, err
	}

	if fitsSocketPath(realDir, port) {
		return realDir, true, nil
	}

	h := fnv.New32a()
	h.Write([]byte(realDir))
	link := filepath.Join(os.TempDir(), fmt.Sprintf("pachyderm-%x", h.Sum32()))

	if target, err := os.Readlink(link); err != nil || target != realDir {
		os.Remove(link)
		if err := os.Symlink(realDir, link); err != nil {
			return "", false, err
		}
	}

	if fitsSocketPath(link, port) {
		return link, true, nil
	}
	return "", false, nil
}

// fitsSocketPath reports whether dir is short enough that postgres's
// ".s.PGSQL.<port>" socket file inside it stays under the kernel's
// Unix-domain socket path limit -- 108 bytes on Linux, 104 on macOS/BSD,
// both including a null terminator. maxSocketPathLen stays comfortably
// under either.
func fitsSocketPath(dir string, port int) bool {
	return len(filepath.Join(dir, fmt.Sprintf(".s.PGSQL.%d", port))) <= maxSocketPathLen
}

// LogFile returns the server log file for a version.
func LogFile(version string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "logs", version+".log"), nil
}

// ConfigFiles is the location of a version's data directory and the config
// files initdb placed inside it.
type ConfigFiles struct {
	DataDir    string `json:"dataDir"`
	ConfigFile string `json:"configFile"`
	HBAFile    string `json:"hbaFile"`
	IdentFile  string `json:"identFile"`
}

// GetConfigFiles returns the paths to a version's postgresql.conf, pg_hba.conf,
// and pg_ident.conf, and the data directory containing them. It errors if the
// data directory has not been initialized yet, since those files don't exist
// until initdb creates them.
func GetConfigFiles(version string) (ConfigFiles, error) {
	initialized, err := IsDataDirInitialized(version)
	if err != nil {
		return ConfigFiles{}, err
	}
	if !initialized {
		return ConfigFiles{}, fmt.Errorf("data directory for %s is not initialized yet", version)
	}

	dir, err := DataDir(version)
	if err != nil {
		return ConfigFiles{}, err
	}

	return ConfigFiles{
		DataDir:    dir,
		ConfigFile: filepath.Join(dir, "postgresql.conf"),
		HBAFile:    filepath.Join(dir, "pg_hba.conf"),
		IdentFile:  filepath.Join(dir, "pg_ident.conf"),
	}, nil
}

func binPath(version, name string) (string, error) {
	installed, err := IsInstalled(version)
	if err != nil {
		return "", err
	}
	if !installed {
		return "", fmt.Errorf("version %s is not installed", version)
	}
	dir, err := InstallDir(version)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "bin", binName(runtime.GOOS, name)), nil
}

// binName appends the platform executable extension to name. Go's
// exec.Command only does PATHEXT/.exe resolution for a bare command name
// with no path separators -- since binPath always builds a full path,
// Windows needs the extension spelled out explicitly, or CreateProcess
// fails to find the file at all (theseus-rs ships pg_ctl.exe, not pg_ctl).
// This was silently masked before Go 1.22, which used to add ".exe"
// implicitly for absolute paths too; that was removed for security
// reasons (see https://go.dev/issue/66586), so this project's Go 1.24+
// toolchain needs it done explicitly.
func binName(goos, name string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

// IsDataDirInitialized reports whether a version's data directory has already
// been initialized with initdb.
func IsDataDirInitialized(version string) (bool, error) {
	dir, err := DataDir(version)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(dir, "PG_VERSION"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InitDB initializes a version's data directory. It is a no-op if the data
// directory is already initialized.
func InitDB(version string) error {
	if initialized, err := IsDataDirInitialized(version); err != nil {
		return err
	} else if initialized {
		return nil
	}

	initdb, err := binPath(version, "initdb")
	if err != nil {
		return err
	}

	dataDir, err := DataDir(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dataDir), 0o755); err != nil {
		return err
	}

	out, err := exec.Command(initdb, "-D", dataDir).CombinedOutput()
	if err != nil {
		os.RemoveAll(dataDir)
		return fmt.Errorf("initdb failed: %w\n%s", err, out)
	}

	if err := ensurePostgresSuperuser(version, dataDir); err != nil {
		os.RemoveAll(dataDir)
		return err
	}

	return nil
}

// ensurePostgresSuperuser adds a "postgres" superuser role on top of
// initdb's own bootstrap superuser, which initdb names after whichever OS
// user ran it rather than "postgres". Without this, connecting as
// "postgres" -- the convention nearly every tool, tutorial, and the
// official Docker image assumes -- fails with "role postgres does not
// exist", even though the cluster works fine otherwise.
func ensurePostgresSuperuser(version, dataDir string) error {
	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("determining current user: %w", err)
	}
	if current.Username == "postgres" {
		return nil // initdb's bootstrap superuser already is named "postgres"
	}

	postgres, err := binPath(version, "postgres")
	if err != nil {
		return err
	}

	// --single (single-user mode) runs one command directly against the
	// data directory with no networking involved, so it needs no port and
	// can't collide with an already-running server.
	cmd := exec.Command(postgres, "--single", "-D", dataDir, "postgres")
	cmd.Stdin = strings.NewReader("CREATE ROLE postgres LOGIN SUPERUSER;\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating postgres superuser role: %w\n%s", err, out)
	}

	return nil
}

// StartServer starts a version's PostgreSQL server on the given port. The
// data directory must already be initialized via InitDB.
func StartServer(version string, port int) error {
	if initialized, err := IsDataDirInitialized(version); err != nil {
		return err
	} else if !initialized {
		return fmt.Errorf("data directory for %s is not initialized yet", version)
	}

	pgCtl, err := binPath(version, "pg_ctl")
	if err != nil {
		return err
	}

	dataDir, err := DataDir(version)
	if err != nil {
		return err
	}

	logFile, err := LogFile(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}

	options := fmt.Sprintf("-p %d -h 127.0.0.1", port)

	// Unix-domain sockets aren't supported by PostgreSQL on Windows, so this
	// is additive everywhere else rather than a replacement for the TCP
	// listener above: it gives psql (or anything else) a socket to connect
	// through without going over loopback, entirely under the user's own
	// home directory instead of the root-owned /var/run/postgresql.
	if runtime.GOOS != "windows" {
		if dir, ok, err := socketConnectDir(version, port); err != nil {
			return err
		} else if ok {
			options += " -k " + dir
		}
	}

	out, err := exec.Command(
		pgCtl, "-D", dataDir, "-l", logFile, "-w",
		"-o", options,
		"start",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL %s: %w\n%s", version, err, out)
	}

	return nil
}

// StopServer stops a version's running PostgreSQL server.
func StopServer(version string) error {
	pgCtl, err := binPath(version, "pg_ctl")
	if err != nil {
		return err
	}

	dataDir, err := DataDir(version)
	if err != nil {
		return err
	}

	out, err := exec.Command(pgCtl, "-D", dataDir, "-m", "fast", "-w", "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop PostgreSQL %s: %w\n%s", version, err, out)
	}

	return nil
}

// TailLog returns up to the last n lines of a version's server log, written
// by pg_ctl each time the server starts.
func TailLog(version string, n int) (string, error) {
	path, err := LogFile(version)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("no log file for %s yet; start the server at least once", version)
	}
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}

var statusPIDRe = regexp.MustCompile(`\(PID:\s*(\d+)\)`)

// ServerStatus reports whether a version's server is running and, if so, its PID.
func ServerStatus(version string) (running bool, pid int, err error) {
	if initialized, ierr := IsDataDirInitialized(version); ierr != nil {
		return false, 0, ierr
	} else if !initialized {
		return false, 0, nil
	}

	pgCtl, err := binPath(version, "pg_ctl")
	if err != nil {
		return false, 0, err
	}

	dataDir, err := DataDir(version)
	if err != nil {
		return false, 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, pgCtl, "-D", dataDir, "status").CombinedOutput()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		return false, 0, fmt.Errorf("checking status of %s: %w\n%s", version, err, out)
	}

	switch exitCode {
	case 0:
		if m := statusPIDRe.FindSubmatch(out); m != nil {
			fmt.Sscanf(string(m[1]), "%d", &pid)
		}
		return true, pid, nil
	case 3:
		return false, 0, nil
	default:
		return false, 0, fmt.Errorf("could not determine status of %s: %s", version, out)
	}
}

// RunningPort returns the port a version's running server is actually
// listening on, read from postmaster.pid (written by postgres itself on
// startup, removed on a clean stop). This is the authoritative source: it
// reflects the port the server was started with even if the configured
// default port has changed since.
func RunningPort(version string) (int, error) {
	dataDir, err := DataDir(version)
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "postmaster.pid"))
	if err != nil {
		return 0, err
	}

	// Line 4 of postmaster.pid is the port number; see PostgreSQL's
	// src/backend/utils/init/miscinit.c (CreateLockFile).
	lines := strings.Split(string(data), "\n")
	if len(lines) < 4 {
		return 0, fmt.Errorf("unexpected postmaster.pid format for %s", version)
	}

	port, err := strconv.Atoi(strings.TrimSpace(lines[3]))
	if err != nil {
		return 0, fmt.Errorf("parsing port from postmaster.pid for %s: %w", version, err)
	}
	return port, nil
}

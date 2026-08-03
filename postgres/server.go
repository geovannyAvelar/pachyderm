package postgres

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// LogFile returns the server log file for a version.
func LogFile(version string) (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "logs", version+".log"), nil
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
	return filepath.Join(dir, "bin", name), nil
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

	out, err := exec.Command(
		pgCtl, "-D", dataDir, "-l", logFile, "-w",
		"-o", fmt.Sprintf("-p %d -h 127.0.0.1", port),
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

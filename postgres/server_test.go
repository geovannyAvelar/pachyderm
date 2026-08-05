package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFakeBin installs a shell script as <version>/bin/<name> so server.go's
// exec.Command calls can be exercised without a real PostgreSQL install.
func writeFakeBin(t *testing.T, version, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binaries are not supported on windows")
	}

	dir, err := InstallDir(version)
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestInitDBCreatesDataDir(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	writeFakeBin(t, version, "initdb", `
for arg in "$@"; do
  prev="$arg"
done
# find the -D argument value (last two args are: -D <dir>)
dir=""
prevflag=""
for arg in "$@"; do
  if [ "$prevflag" = "-D" ]; then
    dir="$arg"
  fi
  prevflag="$arg"
done
mkdir -p "$dir"
echo "16" > "$dir/PG_VERSION"
`)

	if initialized, err := IsDataDirInitialized(version); err != nil || initialized {
		t.Fatalf("expected uninitialized, got initialized=%v err=%v", initialized, err)
	}

	if err := InitDB(version); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	initialized, err := IsDataDirInitialized(version)
	if err != nil {
		t.Fatalf("IsDataDirInitialized: %v", err)
	}
	if !initialized {
		t.Fatal("expected data directory to be initialized after InitDB")
	}

	// Calling InitDB again should be a no-op and not fail.
	if err := InitDB(version); err != nil {
		t.Fatalf("InitDB (second call): %v", err)
	}
}

func TestServerStatusParsing(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	dataDir, err := DataDir(version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("16"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeFakeBin(t, version, "pg_ctl", `
echo "pg_ctl: server is running (PID: 4242)"
exit 0
`)
	running, pid, err := ServerStatus(version)
	if err != nil {
		t.Fatalf("ServerStatus: %v", err)
	}
	if !running || pid != 4242 {
		t.Fatalf("got running=%v pid=%d, want running=true pid=4242", running, pid)
	}

	writeFakeBin(t, version, "pg_ctl", `
echo "pg_ctl: no server running"
exit 3
`)
	running, pid, err = ServerStatus(version)
	if err != nil {
		t.Fatalf("ServerStatus: %v", err)
	}
	if running || pid != 0 {
		t.Fatalf("got running=%v pid=%d, want running=false pid=0", running, pid)
	}
}

func TestServerStatusNotInitialized(t *testing.T) {
	withTempHome(t)

	running, pid, err := ServerStatus("16.14.0")
	if err != nil {
		t.Fatalf("ServerStatus: %v", err)
	}
	if running || pid != 0 {
		t.Fatalf("got running=%v pid=%d, want running=false pid=0", running, pid)
	}
}

func TestGetConfigFiles(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	if _, err := GetConfigFiles(version); err == nil {
		t.Fatal("expected error for uninitialized data directory")
	}

	dataDir, err := DataDir(version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("16"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := GetConfigFiles(version)
	if err != nil {
		t.Fatalf("GetConfigFiles: %v", err)
	}
	if files.DataDir != dataDir {
		t.Errorf("DataDir = %q, want %q", files.DataDir, dataDir)
	}
	if want := filepath.Join(dataDir, "postgresql.conf"); files.ConfigFile != want {
		t.Errorf("ConfigFile = %q, want %q", files.ConfigFile, want)
	}
	if want := filepath.Join(dataDir, "pg_hba.conf"); files.HBAFile != want {
		t.Errorf("HBAFile = %q, want %q", files.HBAFile, want)
	}
	if want := filepath.Join(dataDir, "pg_ident.conf"); files.IdentFile != want {
		t.Errorf("IdentFile = %q, want %q", files.IdentFile, want)
	}
}

func TestStartServerRequiresInitializedDataDir(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	writeFakeBin(t, version, "pg_ctl", `exit 0`)

	if err := StartServer(version, 5432); err == nil {
		t.Fatal("expected error when data directory is not initialized")
	}
}

func TestRunningPort(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	if _, err := RunningPort(version); err == nil {
		t.Fatal("expected error when postmaster.pid does not exist")
	}

	dataDir, err := DataDir(version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pidFile := "4242\n" + dataDir + "\n1700000000\n5433\n/tmp\n*\n"
	if err := os.WriteFile(filepath.Join(dataDir, "postmaster.pid"), []byte(pidFile), 0o644); err != nil {
		t.Fatal(err)
	}

	port, err := RunningPort(version)
	if err != nil {
		t.Fatalf("RunningPort: %v", err)
	}
	if port != 5433 {
		t.Fatalf("RunningPort = %d, want 5433", port)
	}
}

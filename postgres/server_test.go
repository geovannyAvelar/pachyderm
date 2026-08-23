package postgres

import (
	"fmt"
	"hash/fnv"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
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
	writeFakeBin(t, version, "postgres", `exit 0`)

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

func TestSocketDir(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	home, err := HomeDir()
	if err != nil {
		t.Fatal(err)
	}

	dir, err := SocketDir(version)
	if err != nil {
		t.Fatalf("SocketDir: %v", err)
	}
	if want := filepath.Join(home, "run", version); dir != want {
		t.Errorf("SocketDir = %q, want %q", dir, want)
	}
}

func TestBinName(t *testing.T) {
	cases := []struct {
		goos, name, want string
	}{
		{"windows", "pg_ctl", "pg_ctl.exe"},
		{"windows", "psql", "psql.exe"},
		{"linux", "pg_ctl", "pg_ctl"},
		{"darwin", "pg_ctl", "pg_ctl"},
	}

	for _, c := range cases {
		if got := binName(c.goos, c.name); got != c.want {
			t.Errorf("binName(%q, %q) = %q, want %q", c.goos, c.name, got, c.want)
		}
	}
}

func TestInitDBCreatesPostgresSuperuser(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	writeFakeBin(t, version, "initdb", `
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

	captureFile := filepath.Join(t.TempDir(), "postgres-invocation")
	writeFakeBin(t, version, "postgres", `
echo "$@" > `+captureFile+`
cat >> `+captureFile+`
exit 0
`)

	if err := InitDB(version); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	captured, err := os.ReadFile(captureFile)
	if err != nil {
		// The test process is virtually never running as a user actually
		// named "postgres", but skip rather than fail if it somehow is --
		// ensurePostgresSuperuser correctly does nothing in that case.
		if current, uerr := user.Current(); uerr == nil && current.Username == "postgres" {
			t.Skip("test running as the postgres user; no superuser bootstrap expected")
		}
		t.Fatalf("expected the fake postgres binary to run: %v", err)
	}

	got := string(captured)
	if !strings.Contains(got, "--single") {
		t.Errorf("expected --single (single-user mode) in invocation, got: %q", got)
	}
	if !strings.Contains(got, "CREATE ROLE postgres") {
		t.Errorf("expected a CREATE ROLE postgres statement on stdin, got: %q", got)
	}
}

func TestSocketConnectDirShortPath(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"

	realDir, err := SocketDir(version)
	if err != nil {
		t.Fatal(err)
	}

	dir, ok, err := socketConnectDir(version, 5432)
	if err != nil {
		t.Fatalf("socketConnectDir: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for a short path")
	}
	if dir != realDir {
		t.Errorf("dir = %q, want the real SocketDir %q (no symlink needed)", dir, realDir)
	}
	if _, err := os.Stat(realDir); err != nil {
		t.Errorf("expected SocketDir to be created: %v", err)
	}
}

func TestSocketConnectDirLongPathUsesSymlink(t *testing.T) {
	withTempHome(t)
	// Long enough that HomeDir()/run/<version> exceeds maxSocketPathLen
	// regardless of where the test's temp $HOME happens to live.
	version := strings.Repeat("v", 120)

	realDir, err := SocketDir(version)
	if err != nil {
		t.Fatal(err)
	}
	if fitsSocketPath(realDir, 5432) {
		t.Fatalf("test setup bug: realDir %q unexpectedly fits", realDir)
	}

	dir, ok, err := socketConnectDir(version, 5432)
	if err != nil {
		t.Fatalf("socketConnectDir: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true via a short symlink")
	}
	if dir == realDir {
		t.Fatal("expected a symlink path, not the (too long) real dir")
	}
	if !fitsSocketPath(dir, 5432) {
		t.Errorf("returned dir %q still doesn't fit", dir)
	}
	t.Cleanup(func() { os.Remove(dir) })

	target, err := os.Readlink(dir)
	if err != nil {
		t.Fatalf("expected %q to be a symlink: %v", dir, err)
	}
	if target != realDir {
		t.Errorf("symlink target = %q, want %q", target, realDir)
	}

	// Calling again should reuse the existing, already-correct symlink
	// rather than erroring or creating a different one.
	dir2, ok2, err := socketConnectDir(version, 5432)
	if err != nil || !ok2 {
		t.Fatalf("second call: dir=%q ok=%v err=%v", dir2, ok2, err)
	}
	if dir2 != dir {
		t.Errorf("second call returned %q, want the same symlink %q", dir2, dir)
	}
}

func TestSocketConnectDirReplacesStaleSymlink(t *testing.T) {
	withTempHome(t)
	version := strings.Repeat("v", 120)

	realDir, err := SocketDir(version)
	if err != nil {
		t.Fatal(err)
	}

	h := fnv.New32a()
	h.Write([]byte(realDir))
	link := filepath.Join(os.TempDir(), fmt.Sprintf("pachyderm-%x", h.Sum32()))
	t.Cleanup(func() { os.Remove(link) })

	stale := t.TempDir()
	if err := os.Symlink(stale, link); err != nil {
		t.Fatal(err)
	}

	dir, ok, err := socketConnectDir(version, 5432)
	if err != nil || !ok {
		t.Fatalf("dir=%q ok=%v err=%v", dir, ok, err)
	}
	if dir != link {
		t.Fatalf("dir = %q, want the fixed-up symlink %q", dir, link)
	}

	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if target != realDir {
		t.Errorf("stale symlink was not replaced: target = %q, want %q", target, realDir)
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

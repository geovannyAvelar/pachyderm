package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedDataDir creates an initialized-looking data directory at data/<name>.
func seedDataDir(t *testing.T, name string) string {
	t.Helper()
	root, err := dataRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PG_VERSION"), []byte(MajorVersion(name)), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMajorVersion(t *testing.T) {
	cases := map[string]string{
		"16.14.0": "16",
		"16.10":   "16",
		"16":      "16",
		"18.7.0":  "18",
		"9.6.24":  "9.6",
		"9.6":     "9.6",
	}
	for in, want := range cases {
		if got := MajorVersion(in); got != want {
			t.Errorf("MajorVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDataDirIsSharedAcrossMinorReleases(t *testing.T) {
	home := withTempHome(t)

	a, err := DataDir("16.14.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := DataDir("16.16.0")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("DataDir differs across minor releases: %q vs %q", a, b)
	}
	if want := filepath.Join(home, ".pachyderm", "data", "16"); a != want {
		t.Fatalf("DataDir = %q, want %q", a, want)
	}

	other, err := DataDir("17.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if other == a {
		t.Fatal("expected a different data directory for another major version")
	}
}

func TestMigrateLegacyDataDirsMovesSingleDir(t *testing.T) {
	withTempHome(t)
	legacy := seedDataDir(t, "16.14.0")
	if err := os.WriteFile(filepath.Join(legacy, "marker"), []byte("my data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetPort("17", 5440); err != nil { // creates config.json
		t.Fatal(err)
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Ports["16.14.0"] = 5433 // a port an older release saved per release
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	notices, err := MigrateLegacyDataDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyDataDirs: %v", err)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "Moved") {
		t.Fatalf("notices = %q, want one 'Moved' notice", notices)
	}

	dir, err := DataDir("16.16.0")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "marker"))
	if err != nil || string(data) != "my data" {
		t.Fatalf("expected legacy data at %s, got %q, %v", dir, data, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("expected legacy dir to be gone, stat err = %v", err)
	}

	port, err := GetPort("16.16.0")
	if err != nil {
		t.Fatal(err)
	}
	if port != 5433 {
		t.Fatalf("GetPort after migration = %d, want 5433", port)
	}
	if port, _ := GetPort("17.1.0"); port != 5440 {
		t.Fatalf("unrelated port changed: got %d, want 5440", port)
	}

	// A second run has nothing left to do.
	notices, err = MigrateLegacyDataDirs()
	if err != nil || len(notices) != 0 {
		t.Fatalf("second run: notices = %q, err = %v", notices, err)
	}
}

func TestMigrateLegacyDataDirsLeavesAmbiguousDirs(t *testing.T) {
	withTempHome(t)
	a := seedDataDir(t, "16.14.0")
	b := seedDataDir(t, "16.16.0")

	notices, err := MigrateLegacyDataDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyDataDirs: %v", err)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "Rename") {
		t.Fatalf("notices = %q, want one 'Rename' notice", notices)
	}
	for _, dir := range []string{a, b} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected %s to be left in place: %v", dir, err)
		}
	}
	if initialized, _ := IsDataDirInitialized("16.16.0"); initialized {
		t.Fatal("expected no per-major data directory to be created")
	}
}

func TestMigrateLegacyDataDirsKeepsExistingMajorDir(t *testing.T) {
	withTempHome(t)
	legacy := seedDataDir(t, "16.14.0")
	seedDataDir(t, "16")

	notices, err := MigrateLegacyDataDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyDataDirs: %v", err)
	}
	if len(notices) != 0 {
		t.Fatalf("notices = %q, want none", notices)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("expected legacy dir to be left in place: %v", err)
	}
}

func TestMigrateLegacyDataDirsSkipsRunningServer(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"
	legacy := seedDataDir(t, version)
	if err := os.WriteFile(filepath.Join(legacy, "postmaster.pid"), []byte("4242\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFakeBin(t, version, "pg_ctl", `echo "pg_ctl: server is running (PID: 4242)"; exit 0`)

	notices, err := MigrateLegacyDataDirs()
	if err != nil {
		t.Fatalf("MigrateLegacyDataDirs: %v", err)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "Stop it") {
		t.Fatalf("notices = %q, want one 'Stop it' notice", notices)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("expected running legacy dir to be left in place: %v", err)
	}
}

func TestInitDBMigratesLegacyDataDir(t *testing.T) {
	withTempHome(t)
	seedDataDir(t, "16.14.0")
	// initdb must not run: the migrated directory is already initialized.
	writeFakeBin(t, "16.16.0", "initdb", `exit 1`)
	writeFakeBin(t, "16.16.0", "postgres", `exit 0`)

	if err := InitDB("16.16.0"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if initialized, _ := IsDataDirInitialized("16.16.0"); !initialized {
		t.Fatal("expected the migrated data directory to be initialized")
	}
}

func TestInitDBRefusesWhenLegacyDirsAreAmbiguous(t *testing.T) {
	withTempHome(t)
	seedDataDir(t, "16.14.0")
	seedDataDir(t, "16.15.0")
	writeFakeBin(t, "16.16.0", "initdb", `exit 0`)

	err := InitDB("16.16.0")
	if err == nil || !strings.Contains(err.Error(), "Rename") {
		t.Fatalf("InitDB error = %v, want a 'Rename' error", err)
	}
}

func TestRunningRelease(t *testing.T) {
	withTempHome(t)
	dataDir := seedDataDir(t, "16")

	if got := runningRelease(dataDir); got != "" {
		t.Fatalf("runningRelease without postmaster.opts = %q, want empty", got)
	}

	installDir, err := InstallDir("16.14.0")
	if err != nil {
		t.Fatal(err)
	}
	opts := filepath.Join(installDir, "bin", "postgres") + ` "-D" "` + dataDir + `" "-p" "5432"` + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, "postmaster.opts"), []byte(opts), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runningRelease(dataDir); got != "16.14.0" {
		t.Fatalf("runningRelease = %q, want 16.14.0", got)
	}

	if err := os.WriteFile(filepath.Join(dataDir, "postmaster.opts"), []byte("/usr/lib/postgresql/16/bin/postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runningRelease(dataDir); got != "" {
		t.Fatalf("runningRelease for a foreign binary = %q, want empty", got)
	}
}

func TestServerStatusAndStartWithSharedDataDir(t *testing.T) {
	withTempHome(t)
	dataDir := seedDataDir(t, "16")
	running := `echo "pg_ctl: server is running (PID: 4242)"; exit 0`
	writeFakeBin(t, "16.14.0", "pg_ctl", running)
	writeFakeBin(t, "16.16.0", "pg_ctl", running)
	writeFakeBin(t, "16.16.0", "postgres", `exit 0`)

	installDir, err := InstallDir("16.14.0")
	if err != nil {
		t.Fatal(err)
	}
	opts := filepath.Join(installDir, "bin", "postgres") + ` "-D" "` + dataDir + `"` + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, "postmaster.opts"), []byte(opts), 0o644); err != nil {
		t.Fatal(err)
	}

	if up, _, err := ServerStatus("16.14.0"); err != nil || !up {
		t.Fatalf("ServerStatus(16.14.0) = %v, %v; want running", up, err)
	}
	if up, _, err := ServerStatus("16.16.0"); err != nil || up {
		t.Fatalf("ServerStatus(16.16.0) = %v, %v; want not running (16.14.0 owns the server)", up, err)
	}

	err = StartServer("16.16.0", 5432)
	if err == nil || !strings.Contains(err.Error(), "in use by the PostgreSQL 16.14.0 server") {
		t.Fatalf("StartServer error = %v, want an 'in use' error", err)
	}
}

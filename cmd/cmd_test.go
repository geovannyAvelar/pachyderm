package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"pachyderm/postgres"
)

// fakePgCtl behaves like pg_ctl for start/stop/status against the data
// directory in -D, keeping the server's "state" in postmaster.pid and
// postmaster.opts, the same files a real server writes.
const fakePgCtl = `#!/bin/sh
D=""; O=""; action=""
while [ $# -gt 0 ]; do
  case "$1" in
    -D) D="$2"; shift;;
    -o) O="$2"; shift;;
    -l|-m) shift;;
    -w) ;;
    *) action="$1";;
  esac
  shift
done
case "$action" in
  start)
    port=$(echo "$O" | sed -n 's/.*-p \([0-9]*\).*/\1/p')
    printf '4242\n%s\n0\n%s\n' "$D" "$port" > "$D/postmaster.pid"
    echo "$(dirname "$0")/postgres \"-D\" \"$D\"" > "$D/postmaster.opts";;
  stop) rm -f "$D/postmaster.pid" "$D/postmaster.opts";;
  status)
    if [ -f "$D/postmaster.pid" ]; then echo "pg_ctl: server is running (PID: 4242)"; exit 0; fi
    echo "pg_ctl: no server running"; exit 3;;
esac
`

const fakeInitdb = `#!/bin/sh
while [ $# -gt 0 ]; do [ "$1" = "-D" ] && D="$2"; shift; done
mkdir -p "$D" && echo 16 > "$D/PG_VERSION"
`

// setup gives the test an empty $HOME and working directory, with no
// version requested by the environment.
func setup(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(postgres.VersionEnvVar, "")
	t.Chdir(t.TempDir())
	startPort = 0
	return home
}

// installFake makes version look installed, with fake server binaries.
func installFake(t *testing.T, version string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell-script binaries are not supported on windows")
	}
	dir, err := postgres.InstallDir(version)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"pg_ctl":   fakePgCtl,
		"initdb":   fakeInitdb,
		"postgres": "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// run executes the CLI with args and returns its combined output.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	startPort = 0
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("pachyderm %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func TestServerLifecycle(t *testing.T) {
	setup(t)
	installFake(t, "16.16.0")
	if err := postgres.SetCurrent("16.16.0"); err != nil {
		t.Fatal(err)
	}

	out := mustRun(t, "status")
	if !strings.Contains(out, "16.16.0 (current)") || !strings.Contains(out, "not initialized") {
		t.Fatalf("status before start:\n%s", out)
	}

	// start with no version uses the current one, and initializes it.
	out = mustRun(t, "start", "--port", "5544")
	if !strings.Contains(out, "Initializing the PostgreSQL 16 data directory") || !strings.Contains(out, "Started PostgreSQL 16.16.0 on port 5544") {
		t.Fatalf("start:\n%s", out)
	}
	if port, _ := postgres.GetPort("16.1.0"); port != 5544 {
		t.Fatalf("--port was not saved for the major version: got %d", port)
	}

	out = mustRun(t, "status")
	if !strings.Contains(out, "running") || !strings.Contains(out, "4242") || !strings.Contains(out, "5544") {
		t.Fatalf("status while running:\n%s", out)
	}

	out = mustRun(t, "start", "16")
	if !strings.Contains(out, "already running on port 5544") {
		t.Fatalf("second start:\n%s", out)
	}

	out = mustRun(t, "restart")
	if !strings.Contains(out, "Stopped PostgreSQL 16.16.0") || !strings.Contains(out, "Started PostgreSQL 16.16.0") {
		t.Fatalf("restart:\n%s", out)
	}

	if _, err := run(t, "uninstall", "16.16.0"); err == nil || !strings.Contains(err.Error(), "stop the PostgreSQL") {
		t.Fatalf("uninstall of a running version: err = %v", err)
	}

	out = mustRun(t, "stop")
	if !strings.Contains(out, "Stopped PostgreSQL 16.16.0") {
		t.Fatalf("stop:\n%s", out)
	}
	out = mustRun(t, "stop")
	if !strings.Contains(out, "is not running") {
		t.Fatalf("second stop:\n%s", out)
	}

	out = mustRun(t, "status", "16")
	if !strings.Contains(out, "stopped") || !strings.Contains(out, "5544") {
		t.Fatalf("status after stop:\n%s", out)
	}
}

func TestProjectVersionPicksRelease(t *testing.T) {
	setup(t)
	installFake(t, "15.8.0")
	installFake(t, "16.9.0")
	installFake(t, "16.16.0")
	if err := postgres.SetCurrent("15.8.0"); err != nil {
		t.Fatal(err)
	}

	out := mustRun(t, "current")
	if !strings.Contains(out, "15.8.0 (the active version") {
		t.Fatalf("current with no project file:\n%s", out)
	}

	out = mustRun(t, "local", "16")
	if !strings.Contains(out, "Wrote") {
		t.Fatalf("local:\n%s", out)
	}

	// From a subdirectory, the file in the parent applies, and "16" means
	// the newest installed 16.x (16.16.0, not 16.9.0).
	sub := filepath.Join(".", "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	out = mustRun(t, "current")
	if !strings.Contains(out, "16.16.0 (PostgreSQL 16, from ") || !strings.Contains(out, postgres.ProjectVersionFile) {
		t.Fatalf("current with a project file:\n%s", out)
	}

	out = mustRun(t, "start")
	if !strings.Contains(out, "Started PostgreSQL 16.16.0") {
		t.Fatalf("start in the project:\n%s", out)
	}
	out = mustRun(t, "status", "15")
	if !strings.Contains(out, "15.8.0 (current)") || !strings.Contains(out, "not initialized") {
		t.Fatalf("the current version should be untouched:\n%s", out)
	}
	mustRun(t, "stop")

	t.Setenv(postgres.VersionEnvVar, "15")
	out = mustRun(t, "current")
	if !strings.Contains(out, "15.8.0 (PostgreSQL 15, from "+postgres.VersionEnvVar+")") {
		t.Fatalf("current with %s:\n%s", postgres.VersionEnvVar, out)
	}
}

func TestProjectVersionNotInstalled(t *testing.T) {
	setup(t)
	if err := os.WriteFile(".tool-versions", []byte("postgres 14\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "start")
	if err == nil || !strings.Contains(err.Error(), "PostgreSQL 14 (requested by ") || !strings.Contains(err.Error(), "pachyderm get 14") {
		t.Fatalf("err = %v, want a not-installed error naming the file", err)
	}
}

func TestGetWithoutVersionOrProjectFile(t *testing.T) {
	setup(t)
	if _, err := run(t, "get"); err == nil || !strings.Contains(err.Error(), "no version given") {
		t.Fatalf("err = %v, want a no-version error", err)
	}
}

func TestPsqlRequiresRunningServer(t *testing.T) {
	setup(t)
	installFake(t, "16.16.0")
	dir, err := postgres.InstallDir("16.16.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "psql"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "init", "16")

	if _, err := run(t, "psql", "16", "-c", "select 1"); err == nil || !strings.Contains(err.Error(), "is not running") {
		t.Fatalf("psql err = %v, want a not-running error", err)
	}
	if _, err := run(t, "exec", "16", "no_such_tool"); err == nil || !strings.Contains(err.Error(), "no program named no_such_tool") {
		t.Fatalf("exec err = %v, want a no-such-program error", err)
	}
	if _, err := run(t, "exec", "16", "../../bin/sh"); err == nil || !strings.Contains(err.Error(), "not a PostgreSQL program name") {
		t.Fatalf("exec err = %v, want a bad-name error", err)
	}
}

func TestSplitVersionArg(t *testing.T) {
	cases := []struct {
		in, version, rest []string
	}{
		{nil, nil, nil},
		{[]string{"-c", "select 1"}, nil, []string{"-c", "select 1"}},
		{[]string{"15", "-d", "app"}, []string{"15"}, []string{"-d", "app"}},
		{[]string{"--", "16.4", "pg_dump"}, []string{"16.4"}, []string{"pg_dump"}},
		{[]string{"pg_dump", "15"}, nil, []string{"pg_dump", "15"}},
	}
	for _, c := range cases {
		version, rest := splitVersionArg(c.in)
		if !slices.Equal(version, c.version) || !slices.Equal(rest, c.rest) {
			t.Errorf("splitVersionArg(%q) = %q, %q; want %q, %q", c.in, version, rest, c.version, c.rest)
		}
	}
}

func TestToolEnv(t *testing.T) {
	env := []string{"PATH=/bin", "PGHOST=elsewhere", "PGPORT=1", "PGHOSTADDR=10.0.0.1", "PGUSER=me"}

	if got := toolEnv(env, 0); !slices.Equal(got, env) {
		t.Fatalf("toolEnv with no server changed env: %q", got)
	}

	got := toolEnv(env, 5544)
	want := []string{"PATH=/bin", "PGUSER=me", "PGHOST=127.0.0.1", "PGPORT=5544", "PGDATABASE=postgres"}
	if !slices.Equal(got, want) {
		t.Fatalf("toolEnv = %q, want %q", got, want)
	}

	got = toolEnv([]string{"PGDATABASE=app"}, 5544)
	if !slices.Contains(got, "PGDATABASE=app") || slices.Contains(got, "PGDATABASE=postgres") {
		t.Fatalf("toolEnv overrode an explicit PGDATABASE: %q", got)
	}
}

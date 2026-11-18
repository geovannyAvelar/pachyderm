// Package integration runs the pachyderm CLI end to end inside clean Linux
// containers, as a user would on a fresh machine: a minimal distribution
// image, a non-root user, real PostgreSQL and PostGIS downloads, and real
// servers. It lives in its own module so its testcontainers dependency stays
// out of the CLI's go.mod.
//
// It needs Docker and network access to GitHub:
//
//	cd integration && go test -v -timeout 40m ./...
//
// Set GITHUB_TOKEN to avoid GitHub's unauthenticated API rate limit.
package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// cliPath is the pachyderm binary TestMain builds for the containers.
var cliPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pachyderm-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cliPath = filepath.Join(dir, "pachyderm")
	// The containers run Linux on the Docker host's architecture, which
	// on every supported setup is this machine's own.
	build := exec.Command("go", "build", "-o", cliPath, ".")
	build.Dir = ".."
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building pachyderm: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// distro is a Linux distribution image to run the scenario on, and the
// root commands that give it what any real machine has and a slim image
// lacks: CA certificates (to download over HTTPS) and a regular user
// (initdb refuses to run as root).
type distro struct {
	name  string
	image string
	setup string
}

var distros = []distro{
	{
		name:  "debian",
		image: "debian:bookworm-slim",
		setup: "apt-get update -qq && apt-get install -y -qq ca-certificates >/dev/null && useradd -m tester",
	},
	{
		name:  "ubuntu",
		image: "ubuntu:24.04",
		setup: "apt-get update -qq && apt-get install -y -qq ca-certificates >/dev/null && useradd -m tester",
	},
	{
		name:  "fedora",
		image: "fedora:41",
		setup: "dnf install -y -q ca-certificates shadow-utils >/dev/null && useradd -m tester",
	},
}

// box is one running container, with helpers to run shell scripts in it.
type box struct {
	t *testing.T
	c testcontainers.Container
}

const home = "/home/tester"

func startBox(t *testing.T, d distro) *box {
	t.Helper()
	ctx := context.Background()

	c, err := testcontainers.Run(ctx, d.image,
		testcontainers.WithCmd("sleep", "infinity"),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      cliPath,
			ContainerFilePath: "/usr/local/bin/pachyderm",
			FileMode:          0o755,
		}),
	)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("starting %s: %v", d.image, err)
	}

	b := &box{t: t, c: c}
	b.run("root", d.setup, true)
	return b
}

// run executes script with sh as user, and returns its combined output and
// exit code. With mustSucceed, a non-zero exit fails the test.
func (b *box) run(user, script string, mustSucceed bool) (string, int) {
	b.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dir, env := home, []string{"HOME=" + home, "PATH=/usr/local/bin:/usr/bin:/bin"}
	if user == "root" {
		dir, env = "/", []string{"HOME=/root", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		env = append(env, "GITHUB_TOKEN="+token)
	}

	code, reader, err := b.c.Exec(ctx, []string{"sh", "-c", script},
		tcexec.Multiplexed(),
		tcexec.WithUser(user),
		tcexec.WithWorkingDir(dir),
		tcexec.WithEnv(env),
	)
	if err != nil {
		b.t.Fatalf("running %q: %v", script, err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		b.t.Fatalf("reading output of %q: %v", script, err)
	}

	output := strings.TrimSpace(string(out))
	if mustSucceed && code != 0 {
		b.t.Fatalf("%q exited with %d:\n%s", script, code, output)
	}
	return output, code
}

// sh runs script as the test user and returns its output; it must succeed.
func (b *box) sh(script string) string {
	b.t.Helper()
	out, _ := b.run("tester", script, true)
	return out
}

// shFails runs script as the test user, which must fail, and returns its
// output.
func (b *box) shFails(script string) string {
	b.t.Helper()
	out, code := b.run("tester", script, false)
	if code == 0 {
		b.t.Fatalf("%q succeeded, want it to fail:\n%s", script, out)
	}
	return out
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output doesn't contain %q:\n%s", want, got)
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// lineWith returns the first line of out that contains substr.
func lineWith(t *testing.T, out, substr string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no line contains %q:\n%s", substr, out)
	return ""
}

// step runs one stage of a scenario as a subtest, and stops the scenario
// if it fails: every stage builds on the state the previous ones left.
func step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}

func TestOnCleanDistros(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	for _, d := range distros {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			b := startBox(t, d)
			runScenario(t, b)
		})
	}
}

// runScenario walks through what a user does with pachyderm, from a fresh
// install to several servers, upgrades, projects, and extensions.
func runScenario(t *testing.T, b0 *box) {
	// Subtests get their own *testing.T, so each one gets a box using it.
	on := func(t *testing.T) *box { return &box{t: t, c: b0.c} }

	step(t, "get installs and activates a release", func(t *testing.T) {
		b := on(t)
		out := b.sh("pachyderm get 17")
		assertContains(t, out, "as the active version")
		assertContains(t, b.sh("~/.pachyderm/current/bin/postgres --version"), "(PostgreSQL) 17.")
		// Normal output must go to stdout, so it can be piped.
		assertContains(t, b.sh("pachyderm list --installed 2>/dev/null"), "(current)")
		assertContains(t, b.sh("pachyderm current 2>/dev/null | cut -d' ' -f1"), "17.")
	})

	step(t, "start initializes and runs a server", func(t *testing.T) {
		b := on(t)
		out := b.sh("pachyderm start")
		assertContains(t, out, "Initializing the PostgreSQL 17 data directory")
		assertContains(t, out, "on port 5432")
		assertContains(t, b.sh("pachyderm status"), "running")
		assertEqual(t, b.sh("pachyderm psql -tAc 'SELECT 40 + 2'"), "42")
		b.sh(`pachyderm psql -c "CREATE TABLE notes (body text); INSERT INTO notes VALUES ('kept')"`)
	})

	step(t, "a postgres superuser role exists", func(t *testing.T) {
		b := on(t)
		assertEqual(t, b.sh("pachyderm psql -U postgres -tAc 'SELECT current_user'"), "postgres")
		assertEqual(t, b.sh("pachyderm psql -U postgres -tAc 'SELECT rolsuper FROM pg_roles WHERE rolname = current_user'"), "t")
	})

	step(t, "the server listens on a Unix socket under HOME", func(t *testing.T) {
		b := on(t)
		release := b.sh("pachyderm current | cut -d' ' -f1")
		out := b.sh(fmt.Sprintf("~/.pachyderm/current/bin/psql -h ~/.pachyderm/run/%s -p 5432 -d postgres -tAc 'SELECT 1'", release))
		assertEqual(t, out, "1")
	})

	step(t, "exec runs other programs against the server", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm exec createdb app")
		assertEqual(t, b.sh("pachyderm psql -d app -tAc 'SELECT current_database()'"), "app")
		assertContains(t, b.sh("pachyderm exec pg_dump -t notes postgres"), "CREATE TABLE public.notes")
	})

	step(t, "contrib extensions install", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm extension install 17 pgcrypto")
		assertEqual(t, b.sh("pachyderm psql -tAc \"SELECT length(digest('x', 'sha256'))\""), "32")
		assertContains(t, b.sh("pachyderm extension list 17 | grep '^pgcrypto '"), "yes")
	})

	step(t, "PostGIS downloads and works", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm extension get-postgis 17")
		b.sh("pachyderm extension install 17 postgis")
		assertEqual(t, b.sh("pachyderm psql -tAc 'SELECT ST_Distance(ST_Point(0, 0), ST_Point(3, 4))'"), "5")
		assertContains(t, b.sh("pachyderm psql -tAc 'SELECT postgis_lib_version()'"), "3.")
	})

	step(t, "uninstall refuses a running version", func(t *testing.T) {
		b := on(t)
		assertContains(t, b.shFails("pachyderm uninstall 17"), "stop the PostgreSQL")
	})

	step(t, "restart keeps data", func(t *testing.T) {
		b := on(t)
		out := b.sh("pachyderm restart")
		assertContains(t, out, "Stopped PostgreSQL 17.")
		assertContains(t, out, "Started PostgreSQL 17.")
		assertEqual(t, b.sh("pachyderm psql -tAc 'SELECT body FROM notes'"), "kept")
	})

	step(t, "a data directory from an older release is migrated", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm stop")
		// Recreate the layout older pachyderm releases used: one data
		// directory per release instead of per major version.
		release := b.sh("pachyderm current | cut -d' ' -f1")
		b.sh("mv ~/.pachyderm/data/17 ~/.pachyderm/data/" + release)

		assertContains(t, b.sh("pachyderm status"), "Moved the PostgreSQL "+release+" data directory")
		b.sh("pachyderm start")
		assertEqual(t, b.sh("pachyderm psql -tAc 'SELECT body FROM notes'"), "kept")
	})

	step(t, "a minor upgrade keeps the data", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm get 16.15")
		assertContains(t, b.sh("pachyderm start 16.15 --port 5433"), "Started PostgreSQL 16.15.")
		b.sh(`pachyderm psql 16.15 -c "CREATE TABLE t16 (v int); INSERT INTO t16 VALUES (16)"`)
		b.sh("pachyderm stop 16.15")

		b.sh("pachyderm get 16")
		release := b.sh("pachyderm current | cut -d' ' -f1")
		if strings.HasPrefix(release, "16.15.") {
			t.Fatalf("pachyderm get 16 activated %s, want a newer 16.x", release)
		}
		assertContains(t, b.sh("pachyderm start 16"), "Started PostgreSQL "+release+" on port 5433")
		assertEqual(t, b.sh("pachyderm psql 16 -tAc 'SELECT v FROM t16'"), "16")

		// The older release shares the data directory, but the running
		// server is the newer one's.
		status := b.sh("pachyderm status")
		assertContains(t, lineWith(t, status, release), "running")
		assertContains(t, lineWith(t, status, "16.15."), "stopped")
	})

	step(t, "a project file picks the version", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm use 17")
		b.sh("mkdir -p ~/myapp/src && cd ~/myapp && pachyderm local 16")

		assertContains(t, b.sh("cd ~/myapp/src && pachyderm current"), ".pachyderm-version")
		assertContains(t, b.sh("cd ~/myapp/src && pachyderm psql -tAc 'SHOW server_version'"), "16.")
		assertContains(t, b.sh("pachyderm psql -tAc 'SHOW server_version'"), "17.")
		assertContains(t, b.sh("cd ~/myapp && PACHYDERM_VERSION=17 pachyderm psql -tAc 'SHOW server_version'"), "17.")

		b.sh("mkdir -p ~/asdfapp && echo 'postgres 16' > ~/asdfapp/.tool-versions")
		assertContains(t, b.sh("cd ~/asdfapp && pachyderm psql -tAc 'SHOW server_version'"), "16.")
	})

	step(t, "everything stops cleanly", func(t *testing.T) {
		b := on(t)
		b.sh("pachyderm stop 17 && pachyderm stop 16")
		status := b.sh("pachyderm status")
		if strings.Contains(status, "running") {
			t.Fatalf("a server is still running:\n%s", status)
		}
		b.sh("pachyderm uninstall 16.15.1")
		// The shared data directory outlives the uninstalled release.
		assertEqual(t, b.sh("cat ~/.pachyderm/data/16/PG_VERSION"), "16")
	})
}

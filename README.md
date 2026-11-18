# Pachyderm 🐘

A version manager for PostgreSQL — install, switch between, and run multiple PostgreSQL versions locally, just as [asdf](https://asdf-vm.com/) or [nvm](https://github.com/nvm-sh/nvm) manage other runtimes.

Pachyderm ships two ways to use it:

- **`pachyderm`** — a CLI for installing and switching versions.
- **Pachyderm.app** — a small desktop companion app (in [`gui/`](gui/)) for starting/stopping a local server and running `psql`, similar to [Postgres.app](https://postgresapp.com/).

Both share the same underlying [`postgres`](postgres/) package and install PostgreSQL binaries from [theseus-rs/postgresql-binaries](https://github.com/theseus-rs/postgresql-binaries) — no compiling from source, no system package manager required, and no `sudo`/administrator rights needed at any point, on any OS.

## How it works

Installed versions live under `~/.pachyderm/versions/<version>/`, and `~/.pachyderm/current` is a symlink to whichever version is active:

```
~/.pachyderm/
├── versions/
│   ├── 16.14.0/
│   └── 15.18.0/
├── current -> versions/16.14.0
├── data/<major>/       # PostgreSQL data directories, one per major version (created by `initdb` / the app)
└── logs/<version>.log  # server logs
```

Every release of a major version shares one data directory: `16.14.0` and `16.15.0` both use `data/16`, because PostgreSQL keeps the same on-disk format within a major version. So moving to a newer minor release keeps your databases — stop the server, then start it again with the new release. A different major version (e.g. `17`) has its own data directory; moving data between majors still needs `pg_dump`/`pg_restore` or `pg_upgrade`.

Releases before 0.0.21 kept one data directory per release (`data/16.14.0`). Pachyderm moves such a directory to `data/16` automatically the first time it runs. It never overwrites or deletes data to do so: if the server is still running from the old directory, or a major version has more than one old directory, it prints what to do and leaves them as they are.

Add `~/.pachyderm/current/bin` to your `PATH` once, and switching the active version (via `get`/`use`, or the app) instantly changes what `psql`, `pg_ctl`, etc. resolve to — no need to touch your shell config again.

```bash
export PATH="$HOME/.pachyderm/current/bin:$PATH"
```

Everything lives under the current user's home directory, and every operation — installing a version, switching `current`, initializing a data directory, starting a server — only ever touches that directory. Nothing is written to a system-wide location, so none of it ever needs `sudo` or an administrator prompt.

**On Windows**, that mostly falls out for free, with one exception: creating a directory symlink (which is what `current` is on Linux/macOS) requires `SeCreateSymbolicLinkPrivilege`, a permission standard Windows accounts don't have unless Developer Mode is turned on. So on Windows, `current` is an [NTFS junction](https://learn.microsoft.com/en-us/windows/win32/fileio/hard-links-and-junctions) instead of a symlink — junctions only need write access to `~/.pachyderm`, not that privilege, so no elevation is required. Everything that reads `current` (`CurrentBinDir`, `CurrentVersion`, etc.) works the same either way; junctions are handled transparently by Go's standard library. The desktop app's "start at login" toggle is similarly scoped to the current user: it writes to the `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` registry key rather than registering a system service, which is also admin-free.

## System requirements

Pachyderm itself runs anywhere Go does, but the PostgreSQL binaries it installs (from [theseus-rs/postgresql-binaries](https://github.com/theseus-rs/postgresql-binaries)) have requirements of their own on Linux:

- **glibc 2.36 or newer.** The libraries those builds ship with (`libstdc++`, `libcrypt`, ...) need it, so PostgreSQL doesn't start on older distributions: it works on Debian 12+, Ubuntu 23.04+ (e.g. 24.04), and Fedora 37+, but **not on Ubuntu 22.04 or RHEL / Rocky / AlmaLinux 9** (glibc 2.35 and 2.34). Check yours with `getconf GNU_LIBC_VERSION`.
- **PostgreSQL 13 needs some system libraries.** Unlike 14 and newer, its build isn't self-contained: it uses the system's ICU 72, OpenSSL 3, Kerberos (`libgssapi_krb5`), libxml2, and readline, and won't run where those exact versions are missing (e.g. Debian 13 or Ubuntu 24.04, whose ICU is newer). PostgreSQL 13 is also past its end of life; prefer a newer major.

PostGIS (`pachyderm extension get-postgis`) runs wherever the PostgreSQL it's installed into runs.

## Installing the CLI

Build it from source (requires Go 1.24+):

```bash
go build -o pachyderm .
```

Or download a prebuilt binary from the [Releases page](../../releases) for Linux, macOS, or Windows.

Debian/Ubuntu users can add the project's APT repository instead, which also picks up future updates via `apt upgrade`:

```bash
sudo curl -fsSL https://geovannyavelar.github.io/pachyderm/pubkey.gpg -o /usr/share/keyrings/pachyderm.gpg
echo "deb [signed-by=/usr/share/keyrings/pachyderm.gpg] https://geovannyavelar.github.io/pachyderm stable main" | sudo tee /etc/apt/sources.list.d/pachyderm.list
sudo apt update
sudo apt install pachyderm
```

(the desktop app is in the same repo — `sudo apt install pachyderm-app` — and registers itself in the applications menu).

Or grab the `.deb` directly from a release and install it without adding the repo:

```bash
sudo apt install ./pachyderm_<version>_amd64.deb   # or _arm64.deb
```

## CLI usage

### `pachyderm list`

Show the catalog of PostgreSQL major versions and their support status:

```
$ pachyderm list
VERSION  CURRENT MINOR  SUPPORTED
18       18.7           yes
17       17.12          yes
16       16.16          yes
15       15.20          yes
14       14.25          no
...
```

Add `--installed` to see what's actually installed locally instead:

```
$ pachyderm list --installed
* 16.14.0 (current)
  15.18.0
```

### `pachyderm get <version>`

Download, install, and activate a PostgreSQL version. Accepts a major version, a minor version, or an exact release:

```bash
pachyderm get 16        # latest 16.x
pachyderm get 16.10     # latest build of PostgreSQL 16.10
pachyderm get 16.14.0   # an exact release
```

This resolves the version against real published releases, downloads the right binary for your OS/architecture, checks it against the SHA-256 checksum published with it, extracts it into `~/.pachyderm/versions/<resolved-version>/`, and points `~/.pachyderm/current` at it. The archive is extracted into a temporary directory first and only moved into place once it's complete, so an interrupted download never leaves a half-installed version behind.

### Running a server: `start`, `stop`, `restart`, `status`

```bash
pachyderm start          # start the active version's server (initializing it on first use)
pachyderm start 15 -p 5433   # start PostgreSQL 15 on port 5433, and remember that port
pachyderm status         # every installed version: running or stopped, PID, port
pachyderm restart        # e.g. after editing postgresql.conf
pachyderm stop
```

```
$ pachyderm status
VERSION            STATUS           PID    PORT
15.8.0             stopped          -      5433
16.16.0 (current)  running          41232  5432
17.12.0            not initialized  -      -
```

Servers listen on `127.0.0.1` only. `pachyderm init [version]` initializes a data directory without starting it. Starting a server that's already running, or stopping one that isn't, does nothing, so these are safe to use in scripts.

### `pachyderm psql` and `pachyderm exec`

`psql` connects to a running server with that version's own `psql`, on the `postgres` database unless you name another. Every argument goes to `psql` as it is:

```bash
pachyderm psql
pachyderm psql -c 'SELECT version()'
pachyderm psql 15 -d myapp
```

`exec` runs any other PostgreSQL program of that version, pointed at its server through `PGHOST`/`PGPORT` (and `PGDATABASE=postgres` unless already set):

```bash
pachyderm exec createdb myapp
pachyderm exec pg_dump -Fc myapp -f myapp.dump
```

### Per-project versions: `local` and `current`

```bash
cd ~/code/myapp
pachyderm local 16     # writes .pachyderm-version
pachyderm current      # 16.16.0 (PostgreSQL 16, from /home/user/code/myapp/.pachyderm-version)
pachyderm get          # installs the version the project asks for
pachyderm start        # starts PostgreSQL 16 here, whatever the active version is
```

Every command that takes an optional `[version]` (`start`, `stop`, `restart`, `init`, `psql`, `exec`, `logs`, `config`, and `get`) picks it from the first of:

1. the version given on the command line;
2. the `PACHYDERM_VERSION` environment variable;
3. the nearest `.pachyderm-version` file, or the `postgres` line of an asdf [`.tool-versions`](https://asdf-vm.com/manage/configuration.html#tool-versions) file, in the current directory or a parent;
4. the active (`current`) version.

A major or minor version (`16`, `16.10`) means the newest installed release of it. The project file only affects `pachyderm` commands: `psql` on your `PATH` still comes from `~/.pachyderm/current/bin` — use `pachyderm psql` / `pachyderm exec` to get the project's version.

### `pachyderm use <version>`

Switch the active version to one that's already installed, without re-downloading:

```bash
pachyderm use 15
```

### `pachyderm uninstall <version>`

Remove an installed version:

```bash
pachyderm uninstall 15.18.0
```

Stop its server first; `uninstall` refuses to remove a version whose server is running. The data directory is kept, since other releases of the same major version use it too.

### `pachyderm logs [version]`

Show the server log written by `pg_ctl` each time a version's server starts:

```bash
pachyderm logs 16.14.0           # last 100 lines
pachyderm logs 16.14.0 -n 500    # last 500 lines
pachyderm logs 16.14.0 -f        # follow the log as it grows
```

### `pachyderm config [version]`

Print the data directory and the paths to `postgresql.conf`, `pg_hba.conf`, and `pg_ident.conf` for an installed, initialized version — the same information Postgres.app shows in its "Server Settings" panel:

```
$ pachyderm config 16.14.0
Data directory:   /home/user/.pachyderm/data/16.14.0
postgresql.conf:  /home/user/.pachyderm/data/16.14.0/postgresql.conf
pg_hba.conf:      /home/user/.pachyderm/data/16.14.0/pg_hba.conf
pg_ident.conf:    /home/user/.pachyderm/data/16.14.0/pg_ident.conf
```

### `pachyderm extension`

Enable or disable extensions on a running server — every extension in PostgreSQL's own `contrib` (pgcrypto, hstore, pg_trgm, postgres_fdw, ...) is available:

```bash
pachyderm extension list 17
pachyderm extension install 17 pgcrypto
pachyderm extension uninstall 17 pgcrypto
```

PostGIS isn't part of PostgreSQL, so its binaries are downloaded first, from [geovannyAvelar/postgis-binaries](https://github.com/geovannyAvelar/postgis-binaries), then installed like any other extension:

```bash
pachyderm extension get-postgis 17    # restarts the server if it's running, to load PostGIS
pachyderm extension install 17 postgis
```

PostGIS binaries exist for PostgreSQL 13–18 on Linux (amd64 and arm64), and run wherever the PostgreSQL they're installed into runs. As a safeguard, `get-postgis` checks the glibc version the binaries need before installing anything, and refuses, changing nothing, if the system's is older.

### `pachyderm version`

Print the CLI's own version (also available as `pachyderm --version`):

```bash
$ pachyderm version
0.0.10
```

Release builds have the real version baked in via `-ldflags`; a build from source prints `dev`.

## The desktop app

`gui/` contains a [Wails](https://wails.io/) app that wraps the same logic with a UI: initializing a data directory, starting/stopping a server, opening `psql` in a terminal window, enabling extensions (`CREATE EXTENSION`) on a running server — every extension bundled with PostgreSQL's own `contrib` (pgcrypto, hstore, pg_trgm, postgres_fdw, and so on), the same set Postgres.app ships with — and a "Config" button per version that lists its `postgresql.conf`/`pg_hba.conf`/`pg_ident.conf` paths with a one-click "Reveal" to open each in the OS file manager.

```bash
cd gui
wails dev     # live development
wails build   # production build, output in gui/build/bin
```

See [gui/README.md](gui/README.md) for Wails-specific details.

## Releases

Pushing a version tag (e.g. `v0.0.20`) triggers [`.github/workflows/release.yml`](.github/workflows/release.yml). It first runs the same checks as CI (below), then builds:

| | Linux | macOS | Windows |
|---|---|---|---|
| CLI | amd64, arm64 (`.tar.gz` and `.deb`) | amd64, arm64 (`.tar.gz`) | amd64 (`.exe`) |
| Desktop app | amd64, arm64 (`.tar.gz` and `.deb`) | universal — Intel and Apple Silicon (`.zip`) | amd64 (`.exe`) |

`.deb` packages are built with [nfpm](https://nfpm.goreleaser.com/), configured in [`packaging/`](packaging/). Everything is published to a GitHub Release under that tag. There is no Windows arm64 build, because no prebuilt PostgreSQL binaries exist for it.

### CI

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on every push and pull request: `gofmt` and `go vet`, the CLI's tests on Linux (amd64 and arm64), macOS (Apple Silicon and Intel) and Windows, and the desktop app's tests on Linux, macOS and Windows. On each CLI platform it also installs a real PostgreSQL release with the freshly built CLI and runs it, to check that the binary for that platform downloads, verifies and works.

### Unstable builds

Every push to `main` also publishes an unstable build automatically, via [`.github/workflows/unstable.yml`](.github/workflows/unstable.yml): it computes the next patch version past the latest stable tag, tags that commit `vX.Y.Z-unstable.<short-sha>`, and dispatches `release.yml` to build and publish it exactly like a real release. Only one unstable release ever exists — each run deletes the previous one first — and it's never marked "Latest" on the [Releases page](../../releases), so anything relying on the latest release (including `.../releases/latest`) keeps resolving to the newest actual stable tag.

### The APT repository

The `apt-repo` job in `release.yml` rebuilds the APT repository from scratch on every release — stable or unstable — from every `.deb` ever published to this repo's Releases, and republishes it to GitHub Pages. That means unstable `.deb`s are technically present in the same repository `apt` installs from. They're harmless there: [nfpm](https://nfpm.goreleaser.com/) rewrites a package's `-unstable.<sha>` version suffix to `~unstable.<sha>`, and Debian's version ordering ranks a `~` suffix below the plain version it's attached to, so e.g. `0.0.20~unstable.3cf0bd6` always sorts behind `0.0.20` once that ships. `apt install`/`apt upgrade` therefore always resolve to the newest real release, never an unstable build in between.

## Development

```bash
go build ./...   # from the repo root — builds the CLI
go vet ./...
go test ./...

cd gui
go build ./...   # the gui module has its own go.mod, pointed at the root module via a replace directive
go test ./...
```

### Integration tests

[`integration/`](integration/) runs the CLI end to end with [Testcontainers](https://golang.testcontainers.org/), in clean Debian, Ubuntu, and Fedora containers as a non-root user: installing real PostgreSQL releases, starting servers, `psql`/`exec`, the `postgres` role and the Unix socket, contrib extensions and PostGIS, restarts, migrating an old data directory, a minor upgrade that keeps its data, and project version files. It's a separate Go module, so its dependencies stay out of the CLI's, and it needs Docker and network access to GitHub:

```bash
cd integration
go test -v -timeout 40m ./...   # set GITHUB_TOKEN to avoid GitHub's API rate limit
```

CI runs it on amd64 and arm64.

## License

[MIT](LICENSE)

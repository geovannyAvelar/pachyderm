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
├── data/<version>/     # PostgreSQL data directories (created by `initdb` / the app)
└── logs/<version>.log  # server logs
```

Add `~/.pachyderm/current/bin` to your `PATH` once, and switching the active version (via `get`/`use`, or the app) instantly changes what `psql`, `pg_ctl`, etc. resolve to — no need to touch your shell config again.

```bash
export PATH="$HOME/.pachyderm/current/bin:$PATH"
```

Everything lives under the current user's home directory, and every operation — installing a version, switching `current`, initializing a data directory, starting a server — only ever touches that directory. Nothing is written to a system-wide location, so none of it ever needs `sudo` or an administrator prompt.

**On Windows**, that mostly falls out for free, with one exception: creating a directory symlink (which is what `current` is on Linux/macOS) requires `SeCreateSymbolicLinkPrivilege`, a permission standard Windows accounts don't have unless Developer Mode is turned on. So on Windows, `current` is an [NTFS junction](https://learn.microsoft.com/en-us/windows/win32/fileio/hard-links-and-junctions) instead of a symlink — junctions only need write access to `~/.pachyderm`, not that privilege, so no elevation is required. Everything that reads `current` (`CurrentBinDir`, `CurrentVersion`, etc.) works the same either way; junctions are handled transparently by Go's standard library. The desktop app's "start at login" toggle is similarly scoped to the current user: it writes to the `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` registry key rather than registering a system service, which is also admin-free.

## Installing the CLI

Build it from source (requires Go 1.24+):

```bash
go build -o pachyderm .
```

Or download a prebuilt binary from the [Releases page](../../releases) for Linux, macOS, or Windows.

Debian/Ubuntu users can add the project's APT repository instead, which also picks up future updates via `apt upgrade`:

```bash
sudo curl -fsSL https://geovannyAvelar.github.io/pachyderm/pubkey.gpg -o /usr/share/keyrings/pachyderm.gpg
echo "deb [signed-by=/usr/share/keyrings/pachyderm.gpg] https://geovannyAvelar.github.io/pachyderm stable main" | sudo tee /etc/apt/sources.list.d/pachyderm.list
sudo apt update
sudo apt install pachyderm
```

(the desktop app is in the same repo — `sudo apt install pachyderm-app` — and registers itself in the applications menu).

Or grab the `.deb` directly from a release and install it without adding the repo:

```bash
sudo apt install ./pachyderm_<version>_amd64.deb
```

## CLI usage

### `pachyderm list`

Show the catalog of PostgreSQL major versions and their support status:

```
$ pachyderm list
VERSION  CURRENT MINOR  SUPPORTED
17       17.6           yes
16       16.10          yes
15       15.14          yes
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

This resolves the version against real published releases, downloads the right binary for your OS/architecture, extracts it into `~/.pachyderm/versions/<resolved-version>/`, and points `~/.pachyderm/current` at it.

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

### `pachyderm logs <version>`

Show the server log written by `pg_ctl` each time a version's server starts:

```bash
pachyderm logs 16.14.0           # last 100 lines
pachyderm logs 16.14.0 -n 500    # last 500 lines
pachyderm logs 16.14.0 -f        # follow the log as it grows
```

### `pachyderm config <version>`

Print the data directory and the paths to `postgresql.conf`, `pg_hba.conf`, and `pg_ident.conf` for an installed, initialized version — the same information Postgres.app shows in its "Server Settings" panel:

```
$ pachyderm config 16.14.0
Data directory:   /home/user/.pachyderm/data/16.14.0
postgresql.conf:  /home/user/.pachyderm/data/16.14.0/postgresql.conf
pg_hba.conf:      /home/user/.pachyderm/data/16.14.0/pg_hba.conf
pg_ident.conf:    /home/user/.pachyderm/data/16.14.0/pg_ident.conf
```

### `pachyderm version`

Print the CLI's own version (also available as `pachyderm --version`):

```bash
$ pachyderm version
0.0.10
```

Release builds have the real version baked in via `-ldflags`; a build from source prints `dev`.

## The desktop app

`gui/` contains a [Wails](https://wails.io/) app that wraps the same install/switch logic with a UI for the parts the CLI doesn't do: initializing a data directory, starting/stopping a server, opening `psql`, enabling extensions (`CREATE EXTENSION`) on a running server — every extension bundled with PostgreSQL's own `contrib` (pgcrypto, hstore, pg_trgm, postgres_fdw, and so on), the same set Postgres.app ships with — and a "Config" button per version that lists its `postgresql.conf`/`pg_hba.conf`/`pg_ident.conf` paths with a one-click "Reveal" to open each in the OS file manager.

```bash
cd gui
wails dev     # live development
wails build   # production build, output in gui/build/bin
```

See [gui/README.md](gui/README.md) for Wails-specific details.

## Releases

Pushing a version tag (e.g. `v0.0.1`) triggers [`.github/workflows/release.yml`](.github/workflows/release.yml), which builds the CLI (Linux/macOS/Windows) and the desktop app (Linux/macOS/Windows), plus `.deb` packages for both on Linux (built with [nfpm](https://nfpm.goreleaser.com/), configured in [`packaging/`](packaging/)), and publishes all eight artifacts to a GitHub Release under that tag.

## Development

```bash
go build ./...   # from the repo root — builds the CLI
go vet ./...
go test ./...

cd gui
go build ./...   # the gui module has its own go.mod, pointed at the root module via a replace directive
go test ./...
```

## License

[MIT](LICENSE)

# Pachyderm 🐘

A version manager for PostgreSQL — install, switch between, and run multiple PostgreSQL versions locally, the way [asdf](https://asdf-vm.com/) or [nvm](https://github.com/nvm-sh/nvm) manage other runtimes.

Pachyderm ships two ways to use it:

- **`pachyderm`** — a CLI for installing and switching versions.
- **Pachyderm.app** — a small desktop companion app (in [`gui/`](gui/)) for starting/stopping a local server and running `psql`, similar to [Postgres.app](https://postgresapp.com/).

Both share the same underlying [`postgres`](postgres/) package and install PostgreSQL binaries from [theseus-rs/postgresql-binaries](https://github.com/theseus-rs/postgresql-binaries) — no compiling from source, no system package manager required.

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

## Installing the CLI

Build it from source (requires Go 1.24+):

```bash
go build -o pachyderm .
```

Or download a prebuilt binary from the [Releases page](../../releases) for Linux, macOS, or Windows.

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

## The desktop app

`gui/` contains a [Wails](https://wails.io/) app that wraps the same install/switch logic with a UI for the parts the CLI doesn't do: initializing a data directory, starting/stopping a server, and opening `psql`.

```bash
cd gui
wails dev     # live development
wails build   # production build, output in gui/build/bin
```

See [gui/README.md](gui/README.md) for Wails-specific details.

## Releases

Pushing a version tag (e.g. `v0.0.1`) triggers [`.github/workflows/release.yml`](.github/workflows/release.yml), which builds the CLI (Linux/macOS/Windows) and the desktop app (Linux/macOS/Windows) and publishes all six artifacts to a GitHub Release under that tag.

## Development

```bash
go build ./...   # from the repo root — builds the CLI
go vet ./...
go test ./...

cd gui
go build ./...   # the gui module has its own go.mod, pointed at the root module via a replace directive
go test ./...
```

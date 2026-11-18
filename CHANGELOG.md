# Changelog

All notable changes to this project are documented in this file, based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses
a bespoke `major.minor.patch`-shaped version number rather than Semantic
Versioning proper.

## [Unreleased]

### Changed

- Data directories are now per major version (`~/.pachyderm/data/16`)
  instead of per release (`~/.pachyderm/data/16.14.0`), so moving to a
  newer minor release keeps your databases. Existing per-release data
  directories are moved automatically when that's safe; otherwise
  Pachyderm prints what to do and leaves them as they are.
- Configured server ports are now per major version too.
- A server started by one release of a major version is no longer shown
  as running for the other releases of that major version, and starting
  one of those reports which release is using the data directory.

### Fixed

- Downloads of PostgreSQL and PostGIS binaries are now checked against
  their published SHA-256 checksums before anything is extracted.
- An interrupted or failed install no longer leaves a partial version that
  looked installed: archives are extracted into a staging directory and
  moved into place only when complete.
- `pachyderm uninstall` now refuses to remove a version whose server is
  running, as the desktop app already did.
- `config.json` is written atomically.
- `use 16` with more than one 16.x installed now picks the newest one
  instead of failing as ambiguous.
- Runtime errors no longer print the command's usage after them.
- Updated the version catalog shown by `pachyderm list` (and the
  desktop app): added PostgreSQL 18, updated every supported major's
  current minor, and marked 13 and 14 as no longer supported (14 reached
  end of life on 2026-11-12).
- The macOS desktop app is now a universal binary. The one published as
  `pachyderm-app-macos-amd64.zip` was actually built for Apple Silicon
  only, so it didn't run on Intel Macs; it's now
  `pachyderm-app-macos-universal.zip`.

### Added

- Server commands in the CLI: `init`, `start`, `stop`, `restart`, and
  `status`, which until now only the desktop app had. `start --port`
  saves the port for that major version.
- `pachyderm psql`, connected to a version's running server, and
  `pachyderm exec` to run any other PostgreSQL program (pg_dump,
  createdb, ...) of that version against its server.
- Per-project versions: `pachyderm local <version>` writes a
  `.pachyderm-version` file, and the `postgres` line of an asdf
  `.tool-versions` file and `$PACHYDERM_VERSION` work too. Commands given
  no version use it; `pachyderm current` shows which one applies and why.
- `get`, `logs`, and `config` take an optional version, resolved the same
  way.
- Integration tests (Testcontainers) running the CLI end to end in clean
  Debian, Ubuntu, and Fedora containers, on amd64 and arm64, in CI.
- arm64 builds: the CLI for Linux and macOS (Apple Silicon), and the
  desktop app and `.deb` packages for Linux.
- CI workflow running gofmt, go vet, and the tests on Linux, macOS, and
  Windows (including arm64) on every push and pull request, plus a smoke
  test installing a real PostgreSQL release. Releases now run it first
  and aren't built if it fails.

## [0.0.20] - 2026-09-10

### Added

- Unix-domain socket support for connecting to PostgreSQL servers, with no
  root/admin rights required, falling back to TCP-only on Windows or when
  the socket path would exceed the kernel's length limit.
- Extensions panel: Installed and Available tabs, with search scoped to
  the active tab.

### Changed

- PostGIS now appears in the extensions list like any other extension,
  with a normal Install action that downloads the binary automatically,
  instead of a separate "Get PostGIS" button.
- CI now auto-publishes an unstable release on every push to main.

### Fixed

- `role "postgres" does not exist` when connecting as `-U postgres`: the
  role is now created on already-initialized data directories too (fixed
  up automatically the next time the server starts), not just newly
  initialized ones.
- Windows: postgres binaries are invoked with the `.exe` suffix explicitly,
  fixing lookup failures under this project's Go 1.24+ toolchain.
- Bumped `github.com/labstack/echo/v4` to 4.15.3 in the GUI, fixing a
  high-severity vulnerability.

## [0.0.19] - 2026-08-23

### Added

- PostGIS binary support: download prebuilt binaries from
  [geovannyAvelar/postgis-binaries](https://github.com/geovannyAvelar/postgis-binaries)
  and overlay them onto an installed PostgreSQL version, via a new "Get
  PostGIS" action in the GUI's Extensions panel.
- Extension management on the CLI: `pachyderm extension list|install|uninstall`
  mirror the GUI's extension actions, and `pachyderm extension get-postgis`
  downloads prebuilt PostGIS binaries the same way the GUI does.

## [0.0.18] - 2026-08-11

### Added

- About panel with the app's version and licensing info.

## [0.0.17] - 2026-08-05

### Changed

- Moved PostgreSQL server port configuration from global Settings to
  per-instance.

## [0.0.16] - 2026-08-05

### Fixed

- `apt-repo` release job no longer dies on releases that have no `.deb`
  assets.

## [0.0.15] - 2026-08-05

### Added

- Configurable PostgreSQL server port in the GUI.
- Self-hosted APT repository, published to GitHub Pages on every release.
- Documentation for installing from the APT repository and for the
  no-admin-rights design.

### Fixed

- APT repo hostname casing now matches GitHub Pages' canonical URL.

## [0.0.14] - 2026-08-05

### Added

- Show where each version's config files live, Postgres.app-style.

## [0.0.13] - 2026-08-05

### Fixed

- Setting the current PostgreSQL version on Windows no longer requires
  admin rights.

## [0.0.12] - 2026-08-04

### Changed

- Added spacing between the header buttons.
- Bumped `golang.org/x/net` and `golang.org/x/crypto` in the GUI module.

## [0.0.11] - 2026-08-04

### Added

- Settings panel with an opt-in "start at login" toggle.

## [0.0.10] - 2026-08-04

### Added

- `version` command, and the app's version shown in the GUI.

### Fixed

- Minor wording in the README description.

## [0.0.9] - 2026-08-03

### Added

- A reliable way to close the app from the tray.

## [0.0.8] - 2026-08-03

### Added

- Search box in the Extensions panel.

## [0.0.7] - 2026-08-03

### Changed

- App icon now uses Noto Emoji's elephant.

## [0.0.6] - 2026-08-03

### Fixed

- Tray icon click doing nothing on Linux.

## [0.0.5] - 2026-08-03

### Added

- PostgreSQL extension management in the GUI, Postgres.app-style.

## [0.0.4] - 2026-08-03

### Added

- A way to view a PostgreSQL instance's logs.

## [0.0.3] - 2026-08-03

### Added

- `.deb` packages for the CLI and desktop app.
- Menu-bar/tray icon so the app stays running in the background.

### Changed

- Replaced the default Wails template README with Pachyderm-specific docs.

## [0.0.2] - 2026-08-03

### Fixed

- Preserve the executable permission on Linux/macOS release binaries.

## [0.0.1] - 2026-07-31

### Added

- Initial release: CLI and GUI for installing, switching between, and
  running multiple PostgreSQL versions locally.
- Release workflow publishing CLI and GUI artifacts.

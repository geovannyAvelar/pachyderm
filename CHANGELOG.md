# Changelog

All notable changes to this project are documented in this file, based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses
a bespoke `major.minor.patch`-shaped version number rather than Semantic
Versioning proper.

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

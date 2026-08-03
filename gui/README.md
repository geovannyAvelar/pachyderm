# Pachyderm desktop app

A small [Wails](https://wails.io/) app that wraps the [`postgres`](../postgres/) package with a UI, similar to [Postgres.app](https://postgresapp.com/): install and switch PostgreSQL versions, initialize a data directory, start/stop a local server, open `psql` against it, view its server log, and enable/disable extensions.

See the [root README](../README.md) for what Pachyderm is and how the CLI and app share the same version store under `~/.pachyderm/`.

## Runs in the background

Pachyderm has no Dock/taskbar icon — it lives entirely in the menu bar/tray. It starts hidden with just a tray icon; closing the window hides it instead of quitting. Use the tray menu's "Show Pachyderm" to reopen the window and "Quit Pachyderm" to actually exit.

## Live development

```bash
wails dev
```

Runs a Vite dev server with hot reload for the frontend. A dev server also runs on http://localhost:34115 — connect to it in a browser to call the bound Go methods from devtools.

## Building

```bash
wails build
```

Produces a production build in `build/bin`.

## Extensions

The "Extensions" panel on a running version lists everything `pg_available_extensions` reports — the standard PostgreSQL `contrib` modules bundled with every build (pgcrypto, hstore, pg_trgm, postgres_fdw, unaccent, etc.) — and lets you enable (`CREATE EXTENSION`) or disable (`DROP EXTENSION`) them against the `postgres` database, with a search box to filter by name or description. Some extensions (like `uuid-ossp`) depend on system libraries that may not be installed; enabling them will surface `psql`'s error rather than fail silently.

PostGIS is *not* available here: it isn't part of the standard `contrib` bundle, theseus-rs's prebuilt binaries don't include it, and there's no prebuilt PostGIS binary anywhere compatible with those builds (PostGIS itself only ships via OS package managers, tied to their own PostgreSQL packages, plus it needs GEOS/PROJ/GDAL). Supporting it would mean compiling PostGIS and its dependency chain from source per platform — out of scope for now.

## Layout

- [`app.go`](app.go) — the Wails-bound backend: `ListInstalled`, `ListAvailable`, `Install`, `Use`, `Uninstall`, `InitDataDir`, `StartServer`, `StopServer`, `OpenPsql`, `GetLogs`, `ListExtensions`, `InstallExtension`, `UninstallExtension`. Progress during install is streamed to the frontend via a `log` event.
- [`terminal.go`](terminal.go) — opens a terminal window running `psql` against the active server.
- [`tray.go`](tray.go) / `tray_icon_*.go` — the menu-bar/tray icon and its Show/Quit menu.
- [`frontend/`](frontend/) — a small vanilla JS/Vite UI; see [`frontend/src/main.js`](frontend/src/main.js).

Project settings (name, output filename, frontend build commands) live in [`wails.json`](wails.json); see the [Wails project config docs](https://wails.io/docs/reference/project-config) for details.

## Icon

`build/appicon.png` and `build/windows/icon.ico` (used for the app, Dock/taskbar, and tray icon) are the 🐘 elephant emoji from Google's [Noto Emoji](https://github.com/googlefonts/noto-emoji), licensed under the SIL Open Font License 1.1 / Apache License 2.0 (the project's own docs list both; see its `LICENSE`).

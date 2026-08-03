# Pachyderm desktop app

A small [Wails](https://wails.io/) app that wraps the [`postgres`](../postgres/) package with a UI, similar to [Postgres.app](https://postgresapp.com/): install and switch PostgreSQL versions, initialize a data directory, start/stop a local server, and open `psql` against it.

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

## Layout

- [`app.go`](app.go) — the Wails-bound backend: `ListInstalled`, `ListAvailable`, `Install`, `Use`, `Uninstall`, `InitDataDir`, `StartServer`, `StopServer`, `OpenPsql`. Progress during install is streamed to the frontend via a `log` event.
- [`terminal.go`](terminal.go) — opens a terminal window running `psql` against the active server.
- [`tray.go`](tray.go) / `tray_icon_*.go` — the menu-bar/tray icon and its Show/Quit menu.
- [`frontend/`](frontend/) — a small vanilla JS/Vite UI; see [`frontend/src/main.js`](frontend/src/main.js).

Project settings (name, output filename, frontend build commands) live in [`wails.json`](wails.json); see the [Wails project config docs](https://wails.io/docs/reference/project-config) for details.

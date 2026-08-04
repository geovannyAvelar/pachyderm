package main

import (
	"context"
	"fmt"
	"sync"

	"pachyderm/postgres"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// serverPort is the fixed port used for whichever version is running.
// Only one version can be running at a time in this simple app.
const serverPort = 5432

// logLines is how many lines of a version's server log the app shows.
const logLines = 200

// App is the Wails-bound backend for the Pachyderm companion app.
type App struct {
	ctx context.Context
	mu  sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startTray()
}

func (a *App) emit(message string) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "log", message)
	}
}

// VersionState describes one locally installed PostgreSQL version.
type VersionState struct {
	Version     string `json:"version"`
	Current     bool   `json:"current"`
	Initialized bool   `json:"initialized"`
	Running     bool   `json:"running"`
	PID         int    `json:"pid"`
}

// ListInstalled returns every locally installed version with its live state.
func (a *App) ListInstalled() ([]VersionState, error) {
	versions, err := postgres.ListInstalled()
	if err != nil {
		return nil, err
	}

	current, err := postgres.CurrentVersion()
	if err != nil {
		return nil, err
	}

	states := make([]VersionState, 0, len(versions))
	for _, v := range versions {
		initialized, err := postgres.IsDataDirInitialized(v)
		if err != nil {
			return nil, err
		}

		running, pid, err := postgres.ServerStatus(v)
		if err != nil {
			return nil, err
		}

		states = append(states, VersionState{
			Version:     v,
			Current:     v == current,
			Initialized: initialized,
			Running:     running,
			PID:         pid,
		})
	}

	return states, nil
}

// ListAvailable returns the PostgreSQL major-version catalog.
func (a *App) ListAvailable() ([]postgres.Version, error) {
	return postgres.GetVersions("")
}

// Install downloads, installs, and activates a PostgreSQL version. Progress
// is streamed to the frontend via "log" events.
func (a *App) Install(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	target, err := postgres.Target()
	if err != nil {
		return err
	}

	a.emit(fmt.Sprintf("Looking up PostgreSQL %s releases...", version))
	tags, err := postgres.FetchReleaseTags()
	if err != nil {
		return fmt.Errorf("failed to look up available releases: %w", err)
	}

	tag, err := postgres.SelectTag(tags, version)
	if err != nil {
		return err
	}

	installed, err := postgres.IsInstalled(tag)
	if err != nil {
		return err
	}

	if installed {
		a.emit(fmt.Sprintf("PostgreSQL %s is already installed.", tag))
	} else {
		a.emit(fmt.Sprintf("Downloading PostgreSQL %s (%s)...", tag, target))
		if err := postgres.Install(tag, target); err != nil {
			return fmt.Errorf("failed to install PostgreSQL %s: %w", tag, err)
		}
		a.emit(fmt.Sprintf("Installed PostgreSQL %s.", tag))
	}

	if err := postgres.SetCurrent(tag); err != nil {
		return err
	}
	a.emit(fmt.Sprintf("Set PostgreSQL %s as the active version.", tag))

	return nil
}

// Use switches the active version to an already-installed one.
func (a *App) Use(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return postgres.SetCurrent(version)
}

// Uninstall removes an installed version. It refuses to remove a running version.
func (a *App) Uninstall(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	running, _, err := postgres.ServerStatus(version)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("stop PostgreSQL %s before uninstalling it", version)
	}

	return postgres.Uninstall(version)
}

// InitDataDir initializes a version's data directory.
func (a *App) InitDataDir(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return postgres.InitDB(version)
}

// StartServer starts a version's PostgreSQL server.
func (a *App) StartServer(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return postgres.StartServer(version, serverPort)
}

// StopServer stops a version's running PostgreSQL server.
func (a *App) StopServer(version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return postgres.StopServer(version)
}

// OpenPsql opens a terminal window with psql connected to the running server.
func (a *App) OpenPsql(version string) error {
	if err := requireRunning(version); err != nil {
		return err
	}

	dir, err := postgres.InstallDir(version)
	if err != nil {
		return err
	}

	return openPsqlTerminal(dir, serverPort)
}

// GetLogs returns the tail of a version's server log.
func (a *App) GetLogs(version string) (string, error) {
	return postgres.TailLog(version, logLines)
}

// ListExtensions returns every extension available to a running version's
// server, and whether it's currently installed.
func (a *App) ListExtensions(version string) ([]postgres.Extension, error) {
	if err := requireRunning(version); err != nil {
		return nil, err
	}
	return postgres.ListExtensions(version, serverPort)
}

// InstallExtension enables an extension on a running version's server.
func (a *App) InstallExtension(version, name string) error {
	if err := requireRunning(version); err != nil {
		return err
	}
	return postgres.InstallExtension(version, serverPort, name)
}

// UninstallExtension disables an extension on a running version's server.
func (a *App) UninstallExtension(version, name string) error {
	if err := requireRunning(version); err != nil {
		return err
	}
	return postgres.UninstallExtension(version, serverPort, name)
}

// Quit closes the app. HideWindowOnClose only hides the main window, so the
// tray menu's "Quit Pachyderm" and this are the only ways to actually exit.
func (a *App) Quit() {
	wailsruntime.Quit(a.ctx)
}

// GetAutostartEnabled reports whether Pachyderm is set to start when the
// user logs in. Off by default.
func (a *App) GetAutostartEnabled() (bool, error) {
	return isAutostartEnabled()
}

// SetAutostartEnabled turns starting Pachyderm at login on or off.
func (a *App) SetAutostartEnabled(enabled bool) error {
	return setAutostart(enabled)
}

func requireRunning(version string) error {
	running, _, err := postgres.ServerStatus(version)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("PostgreSQL %s is not running", version)
	}
	return nil
}

package main

// appVersion is set at build time via -ldflags "-X main.appVersion=...". It
// defaults to "dev" for local, non-release builds.
var appVersion = "dev"

// Version returns the Pachyderm app's own version (not a PostgreSQL version).
func (a *App) Version() string {
	return appVersion
}

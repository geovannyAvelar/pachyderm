//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func autostartFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "pachyderm-app.desktop"), nil
}

// isAutostartEnabled reports whether Pachyderm is configured to start when
// the user logs in.
func isAutostartEnabled() (bool, error) {
	path, err := autostartFilePath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// setAutostart enables or disables starting Pachyderm at login, via the XDG
// autostart convention (a .desktop file under ~/.config/autostart).
func setAutostart(enabled bool) error {
	path, err := autostartFilePath()
	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Pachyderm
Comment=Manage local PostgreSQL versions
Exec=%s
Icon=pachyderm-app
Terminal=false
X-GNOME-Autostart-enabled=true
`, exe)

	return os.WriteFile(path, []byte(content), 0o644)
}

//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const autostartLabel = "dev.avelar.pachyderm-app"

func autostartFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist"), nil
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

// setAutostart enables or disables starting Pachyderm at login, via a
// per-user LaunchAgent.
func setAutostart(enabled bool) error {
	path, err := autostartFilePath()
	if err != nil {
		return err
	}

	if !enabled {
		_ = exec.Command("launchctl", "unload", path).Run()
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

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, autostartLabel, exe)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}

	return exec.Command("launchctl", "load", "-w", path).Run()
}

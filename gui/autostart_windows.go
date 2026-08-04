//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const autostartValueName = "Pachyderm"
const autostartKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// isAutostartEnabled reports whether Pachyderm is configured to start when
// the user logs in.
func isAutostartEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer key.Close()

	_, _, err = key.GetStringValue(autostartValueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// setAutostart enables or disables starting Pachyderm at login, via the
// current user's Run registry key.
func setAutostart(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(autostartValueName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return key.SetStringValue(autostartValueName, `"`+exe+`"`)
}

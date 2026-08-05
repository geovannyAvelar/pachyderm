package main

import (
	"os/exec"
	"runtime"
)

// revealInFileManager opens the native file manager with path selected, so
// the user can see a config file (or the data directory) in context.
func revealInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	default: // linux and other unix-likes
		return exec.Command("xdg-open", path).Start()
	}
}

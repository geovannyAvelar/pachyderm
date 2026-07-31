package postgres

import (
	"fmt"
	"runtime"
)

// Target returns the release target triple for the current OS/architecture.
func Target() (string, error) {
	return TargetFor(runtime.GOOS, runtime.GOARCH)
}

// TargetFor maps a Go OS/architecture pair to the target triple used by the
// theseus-rs/postgresql-binaries release assets.
func TargetFor(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return "aarch64-apple-darwin", nil
		case "amd64":
			return "x86_64-apple-darwin", nil
		}
	case "linux":
		switch goarch {
		case "arm64":
			return "aarch64-unknown-linux-gnu", nil
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		}
	}

	return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

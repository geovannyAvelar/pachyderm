//go:build !windows

package postgres

import "os"

// createCurrentLink points link at dir using a symlink.
func createCurrentLink(dir, link string) error {
	return os.Symlink(dir, link)
}

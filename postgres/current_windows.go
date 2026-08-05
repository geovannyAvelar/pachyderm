//go:build windows

package postgres

import (
	"fmt"
	"os/exec"
)

// createCurrentLink points link at dir using an NTFS junction rather than a
// symlink. Creating a symlink requires SeCreateSymbolicLinkPrivilege, which
// standard Windows accounts don't have unless Developer Mode is enabled;
// junctions need no special privilege, only write access to link's parent
// directory, so pachyderm never has to run elevated.
func createCurrentLink(dir, link string) error {
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating junction %s -> %s: %w: %s", link, dir, err, out)
	}
	return nil
}

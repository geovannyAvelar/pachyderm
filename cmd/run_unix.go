//go:build !windows

package cmd

import "syscall"

// runForeground replaces this process with path, so the program owns the
// terminal: Ctrl-C reaches it alone (psql cancels the running query instead
// of exiting), and its exit status becomes pachyderm's.
func runForeground(path string, args, env []string) error {
	return syscall.Exec(path, append([]string{path}, args...), env)
}

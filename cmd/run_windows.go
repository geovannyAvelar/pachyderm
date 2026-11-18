//go:build windows

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// runForeground runs path attached to this console and waits for it.
// Windows has no exec(2), so pachyderm stays as the parent: it ignores
// Ctrl-C while the program runs (the program still gets it, and psql uses
// it to cancel a query), and exits with the program's exit code.
func runForeground(path string, args, env []string) error {
	c := exec.Command(path, args...)
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr

	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	err := c.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// openPsqlTerminal opens a new terminal window running psql against the
// server whose binaries live in installDir, connected over TCP loopback on
// port.
func openPsqlTerminal(installDir string, port int) error {
	psql := filepath.Join(installDir, "bin", "psql")
	args := []string{"-h", "127.0.0.1", "-p", strconv.Itoa(port)}

	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(
			`tell application "Terminal" to do script %q
tell application "Terminal" to activate`,
			shellJoin(psql, args),
		)
		return exec.Command("osascript", "-e", script).Start()

	case "windows":
		cmdArgs := append([]string{"/C", "start", "", "cmd", "/K", psql}, args...)
		return exec.Command("cmd", cmdArgs...).Start()

	default: // linux and other unix-likes
		candidates := [][]string{
			{"x-terminal-emulator", "-e"},
			{"gnome-terminal", "--"},
			{"konsole", "-e"},
			{"xterm", "-e"},
		}
		for _, c := range candidates {
			term := c[0]
			if _, err := exec.LookPath(term); err != nil {
				continue
			}
			cmdArgs := append(append([]string{}, c[1:]...), psql)
			cmdArgs = append(cmdArgs, args...)
			return exec.Command(term, cmdArgs...).Start()
		}
		return fmt.Errorf("no supported terminal emulator found (tried x-terminal-emulator, gnome-terminal, konsole, xterm)")
	}
}

// shellJoin quotes cmd and its args into a single POSIX shell command string.
func shellJoin(cmd string, args []string) string {
	quoted := "'" + escapeSingleQuotes(cmd) + "'"
	for _, a := range args {
		quoted += " '" + escapeSingleQuotes(a) + "'"
	}
	return quoted
}

func escapeSingleQuotes(s string) string {
	out := ""
	for _, r := range s {
		if r == '\'' {
			out += `'\''`
		} else {
			out += string(r)
		}
	}
	return out
}

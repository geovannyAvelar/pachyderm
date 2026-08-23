package cmd

import (
	"fmt"
	"strings"

	"pachyderm/postgres"
)

// resolveInstalledVersion matches a user-supplied version (which may be a
// prefix like "16" or "16.10") against locally installed versions.
func resolveInstalledVersion(version string) (string, error) {
	installed, err := postgres.ListInstalled()
	if err != nil {
		return "", err
	}

	for _, v := range installed {
		if v == version {
			return v, nil
		}
	}

	var matches []string
	prefix := version + "."
	for _, v := range installed {
		if strings.HasPrefix(v, prefix) {
			matches = append(matches, v)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("version %s is not installed; run \"pachyderm get %s\" first", version, version)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("version %s is ambiguous, matches: %s", version, strings.Join(matches, ", "))
	}
}

// resolveRunningVersion resolves version the same way resolveInstalledVersion
// does, and additionally requires the server to be running, returning the
// port it's actually listening on.
func resolveRunningVersion(version string) (string, int, error) {
	resolved, err := resolveInstalledVersion(version)
	if err != nil {
		return "", 0, err
	}

	running, _, err := postgres.ServerStatus(resolved)
	if err != nil {
		return "", 0, err
	}
	if !running {
		return "", 0, fmt.Errorf("PostgreSQL %s is not running", resolved)
	}

	port, err := postgres.RunningPort(resolved)
	if err != nil {
		return "", 0, err
	}

	return resolved, port, nil
}

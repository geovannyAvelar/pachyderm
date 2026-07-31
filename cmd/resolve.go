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

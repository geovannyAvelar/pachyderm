package cmd

import (
	"fmt"
	"os"
	"strings"

	"pachyderm/postgres"
)

// resolveInstalledVersion matches a user-supplied version (which may be a
// prefix like "16" or "16.10") against locally installed versions, and
// fails if more than one matches. Commands that destroy something use it,
// so a short version never picks a release the user didn't mean.
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

// versionSource describes where resolveTargetVersion found a version.
type versionSource struct {
	// Requested is the version as the user wrote it (e.g. "16").
	Requested string
	// From is "argument", the file or environment variable it came from,
	// or "current" for the active version.
	From string
}

// resolveTargetVersion picks the installed release a command acts on, from
// the first of: the command's version argument, PACHYDERM_VERSION, the
// nearest .pachyderm-version or .tool-versions file, or the active
// ("current") version. A version that matches more than one installed
// release resolves to the newest of them.
func resolveTargetVersion(args []string) (string, versionSource, error) {
	if len(args) > 0 {
		release, err := latestInstalled(args[0], "")
		return release, versionSource{Requested: args[0], From: "argument"}, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", versionSource{}, err
	}
	req, ok, err := postgres.FindProjectVersion(wd)
	if err != nil {
		return "", versionSource{}, err
	}
	if ok {
		release, err := latestInstalled(req.Version, req.Source)
		return release, versionSource{Requested: req.Version, From: req.Source}, err
	}

	current, err := postgres.CurrentVersion()
	if err != nil {
		return "", versionSource{}, err
	}
	if current == "" {
		return "", versionSource{}, fmt.Errorf("no PostgreSQL version given, and none is active; run \"pachyderm get <version>\" first")
	}
	return current, versionSource{Requested: current, From: "current"}, nil
}

func latestInstalled(version, source string) (string, error) {
	release, ok, err := postgres.LatestInstalledMatch(version)
	if err != nil {
		return "", err
	}
	if !ok {
		where := ""
		if source != "" {
			where = fmt.Sprintf(" (requested by %s)", source)
		}
		return "", fmt.Errorf("PostgreSQL %s%s is not installed; run \"pachyderm get %s\" first", version, where, version)
	}
	return release, nil
}

// resolveRunningVersion resolves version the same way resolveTargetVersion
// does, and additionally requires the server to be running, returning the
// port it's actually listening on.
func resolveRunningVersion(args []string) (string, int, error) {
	resolved, _, err := resolveTargetVersion(args)
	if err != nil {
		return "", 0, err
	}

	running, _, err := postgres.ServerStatus(resolved)
	if err != nil {
		return "", 0, err
	}
	if !running {
		return "", 0, fmt.Errorf("PostgreSQL %s is not running; start it with \"pachyderm start %s\"", resolved, resolved)
	}

	port, err := postgres.RunningPort(resolved)
	if err != nil {
		return "", 0, err
	}

	return resolved, port, nil
}

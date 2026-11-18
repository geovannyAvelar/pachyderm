package postgres

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectVersionFile is the file "pachyderm local" writes to pin a
// directory tree to a PostgreSQL version, like .nvmrc or .python-version.
const ProjectVersionFile = ".pachyderm-version"

// toolVersionsFile is asdf's version file. Pachyderm reads its "postgres"
// (or "postgresql") line, so a project already set up for asdf works as is.
const toolVersionsFile = ".tool-versions"

// VersionEnvVar overrides every version file when set, for one shell or one
// command: PACHYDERM_VERSION=15 pachyderm psql.
const VersionEnvVar = "PACHYDERM_VERSION"

// VersionRequest is a version a project asks for, and where it was found.
type VersionRequest struct {
	// Version is as written by the user: a major ("16"), a minor ("16.10")
	// or an exact release ("16.14.0").
	Version string
	// Source is the file the version came from, or VersionEnvVar.
	Source string
}

// FindProjectVersion returns the PostgreSQL version requested for dir: the
// PACHYDERM_VERSION environment variable if set, otherwise the nearest
// .pachyderm-version or .tool-versions file in dir or one of its parents
// (in the same directory, .pachyderm-version wins). ok is false if nothing
// requests a version.
func FindProjectVersion(dir string) (req VersionRequest, ok bool, err error) {
	if v := strings.TrimSpace(os.Getenv(VersionEnvVar)); v != "" {
		return VersionRequest{Version: v, Source: VersionEnvVar}, true, nil
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return VersionRequest{}, false, err
	}

	for {
		path := filepath.Join(dir, ProjectVersionFile)
		if v, found, err := readProjectVersionFile(path); err != nil {
			return VersionRequest{}, false, err
		} else if found {
			return VersionRequest{Version: v, Source: path}, true, nil
		}

		path = filepath.Join(dir, toolVersionsFile)
		if v, found, err := readToolVersions(path); err != nil {
			return VersionRequest{}, false, err
		} else if found {
			return VersionRequest{Version: v, Source: path}, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return VersionRequest{}, false, nil
		}
		dir = parent
	}
}

// readProjectVersionFile reads the first non-empty, non-comment line of a
// .pachyderm-version file.
func readProjectVersionFile(path string) (string, bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := validateVersionSpec(line); err != nil {
			return "", false, fmt.Errorf("%s: %w", path, err)
		}
		return line, true, nil
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return "", false, fmt.Errorf("%s has no version in it", path)
}

// readToolVersions returns the version on the "postgres" line of an asdf
// .tool-versions file. asdf allows several versions per tool, in order of
// preference; the first one is used.
func readToolVersions(path string) (string, bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != "postgres" && fields[0] != "postgresql") {
			continue
		}
		if err := validateVersionSpec(fields[1]); err != nil {
			return "", false, fmt.Errorf("%s: %w", path, err)
		}
		return fields[1], true, nil
	}
	return "", false, scanner.Err()
}

// IsVersionSpec reports whether v is shaped like a PostgreSQL version: a
// major ("16"), a minor ("16.10") or an exact release ("16.14.0").
func IsVersionSpec(v string) bool {
	return validateVersionSpec(v) == nil
}

func validateVersionSpec(v string) error {
	if _, err := parseVersionParts(v); err != nil {
		return fmt.Errorf("invalid PostgreSQL version %q", v)
	}
	return nil
}

// WriteProjectVersion pins dir to version by writing its
// .pachyderm-version file, and returns that file's path.
func WriteProjectVersion(dir, version string) (string, error) {
	if err := validateVersionSpec(version); err != nil {
		return "", err
	}
	path := filepath.Join(dir, ProjectVersionFile)
	return path, os.WriteFile(path, []byte(version+"\n"), 0o644)
}

// LatestInstalledMatch returns the newest installed release matching
// version, which may be a major ("16"), a minor ("16.10") or an exact
// release ("16.14.0"). ok is false if no installed release matches.
func LatestInstalledMatch(version string) (release string, ok bool, err error) {
	installed, err := ListInstalled()
	if err != nil {
		return "", false, err
	}
	release, err = SelectTag(installed, version)
	if err != nil {
		return "", false, nil
	}
	return release, true, nil
}

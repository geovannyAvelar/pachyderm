package postgres

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MajorVersion returns the PostgreSQL major version of a release: "16" for
// "16.14.0", and "9.6" for "9.6.24" (before PostgreSQL 10, the major
// version had two components). Releases that share a major version share an
// on-disk data format, so they can all run against the same data directory.
func MajorVersion(version string) string {
	parts := strings.Split(version, ".")
	if n, err := strconv.Atoi(parts[0]); err == nil && n < 10 && len(parts) > 1 {
		return parts[0] + "." + parts[1]
	}
	return parts[0]
}

func dataRoot() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "data"), nil
}

// DataDir returns the data directory for a version's PostgreSQL cluster.
// It is shared by every release of the same major version, so moving from
// 16.14.0 to 16.15.0 keeps the same databases.
func DataDir(version string) (string, error) {
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, MajorVersion(version)), nil
}

// legacyDataDirs returns the names of the initialized per-release data
// directories (e.g. data/16.14.0) that Pachyderm releases before one data
// directory per major version created for major, sorted ascending.
func legacyDataDirs(major string) ([]string, error) {
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == major || MajorVersion(name) != major {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, name, "PG_VERSION")); err == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// MigrateLegacyDataDirs moves every per-release data directory left by older
// Pachyderm releases (data/16.14.0) to its per-major location (data/16).
// It returns one notice per data directory it moved, and per data directory
// it could not move safely and that needs the user's action. It never
// deletes or overwrites data: a major version that already has a per-major
// data directory, or that has more than one legacy one, is left as it is.
func MigrateLegacyDataDirs() ([]string, error) {
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	majors := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() && MajorVersion(e.Name()) != e.Name() {
			majors[MajorVersion(e.Name())] = true
		}
	}

	sorted := make([]string, 0, len(majors))
	for m := range majors {
		sorted = append(sorted, m)
	}
	sort.Strings(sorted)

	var notices []string
	for _, major := range sorted {
		notice, err := migrateLegacyDataDir(major)
		if err != nil {
			return notices, err
		}
		if notice != "" {
			notices = append(notices, notice)
		}
	}
	return notices, nil
}

// migrateLegacyDataDir moves major's single legacy data directory to its
// per-major location. See MigrateLegacyDataDirs.
func migrateLegacyDataDir(major string) (notice string, err error) {
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, major)

	if _, err := os.Stat(target); err == nil {
		return "", nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	legacy, err := legacyDataDirs(major)
	if err != nil {
		return "", err
	}

	switch len(legacy) {
	case 0:
		return "", nil
	case 1:
	default:
		paths := make([]string, len(legacy))
		for i, name := range legacy {
			paths[i] = filepath.Join(root, name)
		}
		return fmt.Sprintf(
			"PostgreSQL %s has more than one data directory from an older Pachyderm release (%s). "+
				"Pachyderm now uses one data directory per major version. "+
				"Rename the one you want to keep to %s.",
			major, strings.Join(paths, ", "), target,
		), nil
	}

	src := filepath.Join(root, legacy[0])
	running, err := legacyServerRunning(major, src)
	if err != nil {
		return "", err
	}
	if running {
		return fmt.Sprintf(
			"The PostgreSQL %s server is running from %s. Stop it, then run Pachyderm again to move its data to %s.",
			legacy[0], src, target,
		), nil
	}

	if err := os.Rename(src, target); err != nil {
		return "", fmt.Errorf("moving %s to %s: %w", src, target, err)
	}
	if err := migrateLegacyPort(legacy[0], major); err != nil {
		return "", err
	}

	return fmt.Sprintf("Moved the PostgreSQL %s data directory to %s.", legacy[0], target), nil
}

// legacyServerRunning reports whether a server is running against dataDir,
// using the pg_ctl of any installed release of major. With no postmaster.pid
// no server is running. With no installed release of major left, no
// Pachyderm-managed server can be running either, so a postmaster.pid there
// is stale.
func legacyServerRunning(major, dataDir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dataDir, "postmaster.pid")); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	installed, err := ListInstalled()
	if err != nil {
		return false, err
	}
	for _, v := range installed {
		if MajorVersion(v) != major {
			continue
		}
		pgCtl, err := binPath(v, "pg_ctl")
		if err != nil {
			return false, err
		}
		if _, err := os.Stat(pgCtl); err != nil {
			continue
		}
		running, _, err := serverStatusAt(pgCtl, dataDir)
		return running, err
	}
	return false, nil
}

// ensureDataDirMigrated moves version's legacy data directory into place if
// needed, and fails with the migration notice if that needs the user's
// action. It keeps InitDB from creating an empty per-major data directory
// next to an unmigrated one that holds the user's databases.
func ensureDataDirMigrated(version string) error {
	notice, err := migrateLegacyDataDir(MajorVersion(version))
	if err != nil {
		return err
	}

	initialized, err := IsDataDirInitialized(version)
	if err != nil {
		return err
	}
	if !initialized {
		if legacy, err := legacyDataDirs(MajorVersion(version)); err != nil {
			return err
		} else if len(legacy) > 0 {
			if notice == "" {
				dir, err := DataDir(version)
				if err != nil {
					return err
				}
				notice = fmt.Sprintf(
					"%s exists but is not an initialized data directory, and PostgreSQL %s has data directories from an older Pachyderm release (%s). "+
						"Remove the empty directory, then try again to move the old data there.",
					dir, MajorVersion(version), strings.Join(legacy, ", "),
				)
			}
			return fmt.Errorf("%s", notice)
		}
	}
	return nil
}

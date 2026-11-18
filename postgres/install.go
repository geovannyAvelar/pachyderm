package postgres

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HomeDir returns pachyderm's data directory, ~/.pachyderm.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pachyderm"), nil
}

// VersionsDir returns the directory holding installed PostgreSQL versions.
func VersionsDir() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "versions"), nil
}

// InstallDir returns the install directory for a specific version.
func InstallDir(version string) (string, error) {
	dir, err := VersionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, version), nil
}

// CurrentLink returns the path of the "current" link.
func CurrentLink() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "current"), nil
}

// CurrentBinDir returns the bin directory reached through the "current" link.
func CurrentBinDir() (string, error) {
	link, err := CurrentLink()
	if err != nil {
		return "", err
	}
	return filepath.Join(link, "bin"), nil
}

// IsInstalled reports whether a version is already installed locally.
func IsInstalled(version string) (bool, error) {
	dir, err := InstallDir(version)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// ListInstalled returns the versions currently installed, sorted ascending.
func ListInstalled() ([]string, error) {
	dir, err := VersionsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, e := range entries {
		// Skip the temporary entries an install in progress creates.
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// Install downloads the release tag/target, checks it against the SHA-256
// checksum published next to it, and extracts it into its install
// directory. It is a no-op if the version is already installed.
//
// The archive is extracted into a staging directory first and renamed into
// place only once it is complete, so an interrupted install never leaves a
// partial version that IsInstalled would report as installed.
func Install(tag, target string) error {
	if installed, err := IsInstalled(tag); err != nil {
		return err
	} else if installed {
		return nil
	}

	dir, err := InstallDir(tag)
	if err != nil {
		return err
	}
	versionsDir := filepath.Dir(dir)
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		return err
	}
	cleanStaleTemp(versionsDir)

	url := AssetURL(tag, target)
	want, err := fetchSHA256File(url + ".sha256")
	if err != nil {
		return fmt.Errorf("fetching checksum: %w", err)
	}

	archive, err := downloadVerified(url, versionsDir, want)
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()

	staging, err := os.MkdirTemp(versionsDir, stagingTempPrefix+tag+"-")
	if err != nil {
		return err
	}
	if err := extractTarGz(archive, staging); err != nil {
		os.RemoveAll(staging)
		return err
	}
	// MkdirTemp creates the directory 0700; match the 0755 of a normal one.
	if err := os.Chmod(staging, 0o755); err != nil {
		os.RemoveAll(staging)
		return err
	}

	if err := os.Rename(staging, dir); err != nil {
		os.RemoveAll(staging)
		// Another process may have installed the same version meanwhile.
		if installed, ierr := IsInstalled(tag); ierr == nil && installed {
			return nil
		}
		return fmt.Errorf("installing %s: %w", tag, err)
	}

	return nil
}

// extractTar extracts a tar stream into destDir, stripping the archive's
// single top-level directory (e.g. postgresql-16.14.0-x86_64-.../).
func extractTar(r io.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	cleanDest := filepath.Clean(destDir)
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		parts := strings.SplitN(header.Name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}

		target := filepath.Join(destDir, parts[1])
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			// A hard link names an earlier entry of the same archive, by
			// its full archive path. The PostgreSQL archives use them for
			// identical files, mostly time zones (UTC, UCT, Etc/UTC, Zulu,
			// ...): skipping them leaves those names missing.
			linkParts := strings.SplitN(header.Linkname, "/", 2)
			if len(linkParts) < 2 || linkParts[1] == "" {
				return fmt.Errorf("archive hard link has an invalid target: %s -> %s", header.Name, header.Linkname)
			}
			source := filepath.Join(destDir, linkParts[1])
			if !strings.HasPrefix(source, cleanDest+string(os.PathSeparator)) {
				return fmt.Errorf("archive hard link escapes destination: %s -> %s", header.Name, header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Link(source, target); err != nil {
				// Some filesystems can't hard-link; a copy is equivalent.
				if err := copyFile(source, target); err != nil {
					return fmt.Errorf("extracting hard link %s: %w", header.Name, err)
				}
			}
		}
	}
}

// copyFile copies src to dst, keeping src's permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// SetCurrent points the "current" link (a symlink on Unix, a junction on Windows) at an installed version.
func SetCurrent(version string) error {
	installed, err := IsInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s is not installed", version)
	}

	dir, err := InstallDir(version)
	if err != nil {
		return err
	}

	link, err := CurrentLink()
	if err != nil {
		return err
	}

	if _, err := os.Lstat(link); err == nil {
		if _, err := os.Readlink(link); err != nil {
			return fmt.Errorf("%s already exists and is not a link managed by pachyderm; remove it manually", link)
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return createCurrentLink(dir, link)
}

// CurrentVersion returns the version the "current" link points to, or ""
// if it has not been set.
func CurrentVersion() (string, error) {
	link, err := CurrentLink()
	if err != nil {
		return "", err
	}

	target, err := os.Readlink(link)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return filepath.Base(target), nil
}

// Uninstall removes an installed version, clearing the "current" link if
// it pointed there. It refuses to remove a version whose server is running.
// The data directory is kept: other releases of the same major version use
// it too.
func Uninstall(version string) error {
	installed, err := IsInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s is not installed", version)
	}

	// A version with no pg_ctl (e.g. a damaged install) can't report its
	// status, and should still be removable.
	if pgCtl, err := binPath(version, "pg_ctl"); err == nil {
		if _, err := os.Stat(pgCtl); err == nil {
			running, _, err := ServerStatus(version)
			if err != nil {
				return err
			}
			if running {
				return fmt.Errorf("stop the PostgreSQL %s server before uninstalling it", version)
			}
		}
	}

	current, err := CurrentVersion()
	if err != nil {
		return err
	}

	dir, err := InstallDir(version)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	if current == version {
		link, err := CurrentLink()
		if err != nil {
			return err
		}
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

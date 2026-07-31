package postgres

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
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

// CurrentLink returns the path of the "current" symlink.
func CurrentLink() (string, error) {
	base, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "current"), nil
}

// CurrentBinDir returns the bin directory reached through the "current" symlink.
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
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// Install downloads and extracts the release tag/target into its install directory.
// It is a no-op if the version is already installed.
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

	url := AssetURL(tag, target)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}
	defer gz.Close()

	if err := extractTar(gz, dir); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("extracting archive: %w", err)
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
		}
	}
}

// SetCurrent points the "current" symlink at an installed version.
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

	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s already exists and is not a symlink managed by pachyderm; remove it manually", link)
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Symlink(dir, link)
}

// CurrentVersion returns the version the "current" symlink points to, or ""
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

// Uninstall removes an installed version, clearing the "current" symlink if
// it pointed there.
func Uninstall(version string) error {
	installed, err := IsInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s is not installed", version)
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

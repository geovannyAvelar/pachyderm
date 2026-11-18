package postgres

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// githubDownloadBase and githubAPIBase are variables only so tests can point
// them at a local server.
var (
	githubDownloadBase = "https://github.com"
	githubAPIBase      = "https://api.github.com"
)

// Prefixes of the temporary files and directories Install and
// InstallPostGISBinary create inside VersionsDir. ListInstalled skips them,
// and cleanStaleTemp removes the ones a crash left behind.
const (
	downloadTempPrefix = ".download-"
	stagingTempPrefix  = ".install-"
)

// staleTempAge is how old a leftover temporary entry must be before
// cleanStaleTemp removes it, so an install running in another process at
// the same time keeps its files.
const staleTempAge = 24 * time.Hour

// fetchSHA256File downloads a checksum file in the "<hex digest>  <file name>"
// format sha256sum writes (the name is optional), and returns the digest.
func fetchSHA256File(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no checksum published at %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}

	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum file at %s", url)
	}
	return normalizeSHA256(fields[0])
}

// parseGitHubDigest returns the hex SHA-256 digest from a GitHub release
// asset's "digest" field ("sha256:<hex>").
func parseGitHubDigest(digest string) (string, error) {
	algo, sum, ok := strings.Cut(digest, ":")
	if !ok || algo != "sha256" {
		return "", fmt.Errorf("unsupported asset digest %q", digest)
	}
	return normalizeSHA256(sum)
}

func normalizeSHA256(sum string) (string, error) {
	sum = strings.ToLower(sum)
	if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
		return "", fmt.Errorf("invalid SHA-256 checksum %q", sum)
	}
	return sum, nil
}

// downloadVerified downloads url into a new temporary file in dir and checks
// its SHA-256 digest against wantSHA256. On success it returns the file,
// open and positioned at its start; the caller closes and removes it. On
// failure nothing is left behind.
func downloadVerified(url, dir, wantSHA256 string) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	f, err := os.CreateTemp(dir, downloadTempPrefix+"*")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return fail(fmt.Errorf("downloading %s: %w", url, err))
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA256 {
		return fail(fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantSHA256))
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	return f, nil
}

// extractTarGz extracts a gzip-compressed tar stream into destDir; see
// extractTar.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}
	defer gz.Close()

	if err := extractTar(gz, destDir); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}
	return nil
}

// cleanStaleTemp removes temporary downloads and staging directories in dir
// that a crashed or interrupted install left behind.
func cleanStaleTemp(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, downloadTempPrefix) && !strings.HasPrefix(name, stagingTempPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < staleTempAge {
			continue
		}
		os.RemoveAll(filepath.Join(dir, name))
	}
}

// mergeDir moves every file and symlink in src into the same relative path
// under dst, replacing what is there, then removes src. src and dst must be
// on the same filesystem.
func mergeDir(src, dst string) error {
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return os.Rename(path, target)
	})
	if err != nil {
		return err
	}
	return os.RemoveAll(src)
}

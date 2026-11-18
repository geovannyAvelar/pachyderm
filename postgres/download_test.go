package postgres

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitHub serves release assets from files (URL path -> content) and
// points githubDownloadBase/githubAPIBase at itself for the test's duration.
func fakeGitHub(t *testing.T, files map[string][]byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	oldDownload, oldAPI := githubDownloadBase, githubAPIBase
	githubDownloadBase, githubAPIBase = srv.URL, srv.URL
	t.Cleanup(func() { githubDownloadBase, githubAPIBase = oldDownload, oldAPI })
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gzipTar(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := buildTar(t, entries).WriteTo(gz); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const testTarget = "x86_64-unknown-linux-gnu"

func pgAssetPath(tag string) string {
	return fmt.Sprintf("/%s/releases/download/%s/postgresql-%s-%s.tar.gz", releasesRepo, tag, tag, testTarget)
}

func assertNoTempLeftovers(t *testing.T) {
	t.Helper()
	dir, err := VersionsDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("leftover temporary entry %s", e.Name())
		}
	}
}

func TestInstallVerifiesChecksum(t *testing.T) {
	withTempHome(t)
	const tag = "16.16.0"
	archive := gzipTar(t, map[string]string{
		"postgresql-16.16.0/bin/pg_ctl": "binary",
	})
	fakeGitHub(t, map[string][]byte{
		pgAssetPath(tag):             archive,
		pgAssetPath(tag) + ".sha256": []byte(sha256Hex(archive) + "  postgresql-16.16.0.tar.gz\n"),
	})

	if err := Install(tag, testTarget); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dir, err := InstallDir(tag)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "bin", "pg_ctl"))
	if err != nil || string(data) != "binary" {
		t.Fatalf("installed file = %q, %v", data, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix permission bits to check.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o755 {
		t.Errorf("install dir mode = %o, want 755", perm)
	}
	assertNoTempLeftovers(t)
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	withTempHome(t)
	const tag = "16.16.0"
	archive := gzipTar(t, map[string]string{
		"postgresql-16.16.0/bin/pg_ctl": "binary",
	})
	fakeGitHub(t, map[string][]byte{
		pgAssetPath(tag):             archive,
		pgAssetPath(tag) + ".sha256": []byte(strings.Repeat("0", 64) + "\n"),
	})

	err := Install(tag, testTarget)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Install error = %v, want a checksum mismatch", err)
	}
	if installed, _ := IsInstalled(tag); installed {
		t.Fatal("expected nothing to be installed after a checksum mismatch")
	}
	assertNoTempLeftovers(t)
}

func TestInstallRequiresChecksum(t *testing.T) {
	withTempHome(t)
	const tag = "16.16.0"
	fakeGitHub(t, map[string][]byte{
		pgAssetPath(tag): gzipTar(t, map[string]string{"postgresql-16.16.0/bin/pg_ctl": "binary"}),
	})

	if err := Install(tag, testTarget); err == nil || !strings.Contains(err.Error(), "no checksum published") {
		t.Fatalf("Install error = %v, want a missing-checksum error", err)
	}
	if installed, _ := IsInstalled(tag); installed {
		t.Fatal("expected nothing to be installed without a checksum")
	}
}

func TestInstallCorruptArchiveLeavesNothing(t *testing.T) {
	withTempHome(t)
	const tag = "16.16.0"
	// A valid gzip header followed by a truncated tar stream: the checksum
	// matches what was published, but extraction fails partway.
	var truncated bytes.Buffer
	gz := gzip.NewWriter(&truncated)
	raw := buildTar(t, map[string]string{"postgresql-16.16.0/bin/pg_ctl": strings.Repeat("x", 4096)}).Bytes()
	gz.Write(raw[:1024])
	gz.Close()
	archive := truncated.Bytes()

	fakeGitHub(t, map[string][]byte{
		pgAssetPath(tag):             archive,
		pgAssetPath(tag) + ".sha256": []byte(sha256Hex(archive)),
	})

	if err := Install(tag, testTarget); err == nil {
		t.Fatal("expected Install to fail on a truncated archive")
	}
	if installed, _ := IsInstalled(tag); installed {
		t.Fatal("expected a failed extraction to leave the version uninstalled")
	}
	assertNoTempLeftovers(t)
}

func TestListInstalledSkipsTemporaryEntries(t *testing.T) {
	withTempHome(t)
	dir, err := VersionsDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"16.16.0", ".install-17.2.0-123", ".download-456"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	versions, err := ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0] != "16.16.0" {
		t.Fatalf("ListInstalled = %v, want [16.16.0]", versions)
	}
}

func TestUninstallRefusesRunningServer(t *testing.T) {
	withTempHome(t)
	const version = "16.14.0"
	seedDataDir(t, MajorVersion(version))
	writeFakeBin(t, version, "pg_ctl", `echo "pg_ctl: server is running (PID: 4242)"; exit 0`)

	err := Uninstall(version)
	if err == nil || !strings.Contains(err.Error(), "stop the PostgreSQL") {
		t.Fatalf("Uninstall error = %v, want a 'stop the server' error", err)
	}
	if installed, _ := IsInstalled(version); !installed {
		t.Fatal("expected the version to stay installed")
	}

	writeFakeBin(t, version, "pg_ctl", fakePgCtlStopped)
	if err := Uninstall(version); err != nil {
		t.Fatalf("Uninstall of a stopped version: %v", err)
	}
	if initialized, _ := IsDataDirInitialized("16.16.0"); !initialized {
		t.Fatal("expected the shared data directory to be kept after uninstall")
	}
}

func TestFetchSHA256FileFormats(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	fakeGitHub(t, map[string][]byte{
		"/plain":     []byte(sum),
		"/sha256sum": []byte(strings.ToUpper(sum) + "  file.tar.gz\n"),
		"/garbage":   []byte("not-a-checksum"),
	})

	for _, path := range []string{"/plain", "/sha256sum"} {
		got, err := fetchSHA256File(githubDownloadBase + path)
		if err != nil || got != sum {
			t.Errorf("fetchSHA256File(%s) = %q, %v; want %q", path, got, err, sum)
		}
	}
	if _, err := fetchSHA256File(githubDownloadBase + "/garbage"); err == nil {
		t.Error("expected an error for an invalid checksum file")
	}
}

func TestInstallPostGISBinaryVerifiesDigest(t *testing.T) {
	withTempHome(t)
	const pgVersion = "16.16.0"
	writeFakeBin(t, pgVersion, "pg_ctl", fakePgCtlStopped)

	target, err := Target()
	if err != nil {
		t.Skip(err)
	}
	archive := gzipTar(t, map[string]string{
		"postgis-3.5.7/lib/postgis-3.so": "postgis",
	})
	assetName := postGISAssetName("v3.5.7", "16", target)

	releases := func(digest string) []byte {
		data, err := json.Marshal([]ghRelease{{
			TagName: "v3.5.7",
			Assets:  []ghAsset{{Name: assetName, Digest: digest}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	assetPath := fmt.Sprintf("/%s/releases/download/v3.5.7/%s", postgisBinariesRepo, assetName)
	apiPath := fmt.Sprintf("/repos/%s/releases", postgisBinariesRepo)
	files := map[string][]byte{
		apiPath:   releases("sha256:" + strings.Repeat("0", 64)),
		assetPath: archive,
	}
	fakeGitHub(t, files)

	dir, err := InstallDir(pgVersion)
	if err != nil {
		t.Fatal(err)
	}
	soPath := filepath.Join(dir, "lib", "postgis-3.so")

	if err := InstallPostGISBinary(pgVersion, "3.5"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("InstallPostGISBinary error = %v, want a checksum mismatch", err)
	}
	if _, err := os.Stat(soPath); !os.IsNotExist(err) {
		t.Fatal("expected a mismatched download to leave the install untouched")
	}

	files[apiPath] = releases("sha256:" + sha256Hex(archive))
	if err := InstallPostGISBinary(pgVersion, "3.5"); err != nil {
		t.Fatalf("InstallPostGISBinary: %v", err)
	}
	data, err := os.ReadFile(soPath)
	if err != nil || string(data) != "postgis" {
		t.Fatalf("PostGIS file = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "pg_ctl")); err != nil {
		t.Fatalf("expected the existing install to be kept: %v", err)
	}
	assertNoTempLeftovers(t)
}

func TestInstallPostGISBinaryRequiresChecksum(t *testing.T) {
	withTempHome(t)
	const pgVersion = "16.16.0"
	writeFakeBin(t, pgVersion, "pg_ctl", fakePgCtlStopped)

	target, err := Target()
	if err != nil {
		t.Skip(err)
	}
	assetName := postGISAssetName("v3.5.7", "16", target)
	data, err := json.Marshal([]ghRelease{{TagName: "v3.5.7", Assets: []ghAsset{{Name: assetName}}}})
	if err != nil {
		t.Fatal(err)
	}
	fakeGitHub(t, map[string][]byte{
		fmt.Sprintf("/repos/%s/releases", postgisBinariesRepo): data,
	})

	if err := InstallPostGISBinary(pgVersion, "3.5"); err == nil || !strings.Contains(err.Error(), "no checksum published") {
		t.Fatalf("InstallPostGISBinary error = %v, want a missing-checksum error", err)
	}
}

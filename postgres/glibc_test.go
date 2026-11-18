package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// dirWithSystemELF returns a directory holding a copy of a dynamically
// linked glibc binary from this system, for tests that need a real ELF.
func dirWithSystemELF(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("needs a glibc ELF binary")
	}
	if _, ok := systemGlibc(); !ok {
		t.Skip("not a glibc system")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile("/bin/ls", filepath.Join(dir, "lib", "libfake.so")); err != nil {
		t.Skipf("no /bin/ls to copy: %v", err)
	}
	return dir
}

func stubSystemGlibc(t *testing.T, version []int, ok bool) {
	t.Helper()
	old := systemGlibc
	systemGlibc = func() ([]int, bool) { return version, ok }
	t.Cleanup(func() { systemGlibc = old })
}

func TestRequiredGlibc(t *testing.T) {
	dir := dirWithSystemELF(t)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("not an ELF file"), 0o644); err != nil {
		t.Fatal(err)
	}

	req, ok, err := requiredGlibc(dir)
	if err != nil || !ok {
		t.Fatalf("requiredGlibc = %v, %v, %v; want a requirement", req, ok, err)
	}
	if req.Version[0] != 2 || req.File != filepath.Join("lib", "libfake.so") {
		t.Fatalf("requirement = %v from %s, want glibc 2.x from lib/libfake.so", req, req.File)
	}

	// /bin/ls runs here, so this system's glibc satisfies it.
	have, _ := systemGlibc()
	if compareParts(have, req.Version) < 0 {
		t.Fatalf("system glibc %v is older than %v, which /bin/ls needs", have, req.Version)
	}
}

func TestRequiredGlibcWithoutELFFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "postgis.control"), []byte("comment = 'x'"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := requiredGlibc(dir); err != nil || ok {
		t.Fatalf("ok=%v err=%v, want no requirement", ok, err)
	}
}

func TestCheckGlibc(t *testing.T) {
	dir := dirWithSystemELF(t)

	stubSystemGlibc(t, []int{1, 0}, true)
	err := checkGlibc(dir, "PostGIS 3.5.7 for PostgreSQL 17")
	if err == nil || !strings.Contains(err.Error(), "needs glibc 2.") || !strings.Contains(err.Error(), "this system has glibc 1.0") || !strings.Contains(err.Error(), "lib/libfake.so") {
		t.Fatalf("checkGlibc on an old system = %v", err)
	}

	stubSystemGlibc(t, []int{99, 0}, true)
	if err := checkGlibc(dir, "x"); err != nil {
		t.Fatalf("checkGlibc on a new system = %v", err)
	}

	stubSystemGlibc(t, nil, false)
	if err := checkGlibc(dir, "x"); err != nil {
		t.Fatalf("checkGlibc with an unknown glibc = %v, want no error", err)
	}
}

func TestInstallPostGISBinaryRefusesNewerGlibc(t *testing.T) {
	elfDir := dirWithSystemELF(t)
	withTempHome(t)
	const pgVersion = "16.16.0"
	writeFakeBin(t, pgVersion, "pg_ctl", fakePgCtlStopped)

	target, err := Target()
	if err != nil {
		t.Skip(err)
	}
	elfBytes, err := os.ReadFile(filepath.Join(elfDir, "lib", "libfake.so"))
	if err != nil {
		t.Fatal(err)
	}
	archive := gzipTar(t, map[string]string{
		"postgis-3.5.7/lib/postgis-3.so": string(elfBytes),
	})
	assetName := postGISAssetName("v3.5.7", "16", target)
	fakeGitHub(t, map[string][]byte{
		"/repos/" + postgisBinariesRepo + "/releases": mustJSON(t, []ghRelease{{
			TagName: "v3.5.7",
			Assets:  []ghAsset{{Name: assetName, Digest: "sha256:" + sha256Hex(archive)}},
		}}),
		"/" + postgisBinariesRepo + "/releases/download/v3.5.7/" + assetName: archive,
	})
	stubSystemGlibc(t, []int{1, 0}, true)

	err = InstallPostGISBinary(pgVersion, "3.5")
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("InstallPostGISBinary = %v, want a glibc refusal", err)
	}
	dir, err := InstallDir(pgVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lib", "postgis-3.so")); !os.IsNotExist(err) {
		t.Fatal("expected the refused PostGIS files to stay out of the install")
	}
	assertNoTempLeftovers(t)
}

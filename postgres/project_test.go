package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindProjectVersion(t *testing.T) {
	t.Setenv(VersionEnvVar, "")
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := FindProjectVersion(nested); err != nil || ok {
		t.Fatalf("with no version file: ok=%v err=%v, want none", ok, err)
	}

	// asdf's file, with other tools and comments around the postgres line.
	toolVersions := filepath.Join(root, ".tool-versions")
	writeFile(t, toolVersions, "nodejs 20.1.0\n# db\npostgres 15.4 14 # fallback\n")
	req, ok, err := FindProjectVersion(nested)
	if err != nil || !ok || req.Version != "15.4" || req.Source != toolVersions {
		t.Fatalf("from .tool-versions: %+v ok=%v err=%v", req, ok, err)
	}

	// .pachyderm-version wins in the same directory.
	pinned := filepath.Join(root, ProjectVersionFile)
	writeFile(t, pinned, "# comment\n\n16\n")
	req, ok, err = FindProjectVersion(nested)
	if err != nil || !ok || req.Version != "16" || req.Source != pinned {
		t.Fatalf("from .pachyderm-version: %+v ok=%v err=%v", req, ok, err)
	}

	// The nearest file wins over one further up.
	closer := filepath.Join(root, "a", ".tool-versions")
	writeFile(t, closer, "postgresql 17.2.0\n")
	req, ok, err = FindProjectVersion(nested)
	if err != nil || !ok || req.Version != "17.2.0" || req.Source != closer {
		t.Fatalf("nearest file: %+v ok=%v err=%v", req, ok, err)
	}

	// The environment variable wins over every file.
	t.Setenv(VersionEnvVar, "13")
	req, ok, err = FindProjectVersion(nested)
	if err != nil || !ok || req.Version != "13" || req.Source != VersionEnvVar {
		t.Fatalf("from %s: %+v ok=%v err=%v", VersionEnvVar, req, ok, err)
	}
}

func TestFindProjectVersionSkipsToolVersionsWithoutPostgres(t *testing.T) {
	t.Setenv(VersionEnvVar, "")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ProjectVersionFile), "16\n")
	sub := filepath.Join(root, "app")
	writeFile(t, filepath.Join(sub, ".tool-versions"), "nodejs 20.1.0\n")

	req, ok, err := FindProjectVersion(sub)
	if err != nil || !ok || req.Version != "16" {
		t.Fatalf("got %+v ok=%v err=%v, want 16 from the parent", req, ok, err)
	}
}

func TestFindProjectVersionRejectsInvalidVersion(t *testing.T) {
	t.Setenv(VersionEnvVar, "")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ProjectVersionFile), "latest\n")

	if _, _, err := FindProjectVersion(root); err == nil || !strings.Contains(err.Error(), "invalid PostgreSQL version") {
		t.Fatalf("err = %v, want an invalid-version error", err)
	}
}

func TestWriteProjectVersion(t *testing.T) {
	t.Setenv(VersionEnvVar, "")
	dir := t.TempDir()

	if _, err := WriteProjectVersion(dir, "not-a-version"); err == nil {
		t.Fatal("expected an error for an invalid version")
	}

	path, err := WriteProjectVersion(dir, "16.10")
	if err != nil {
		t.Fatal(err)
	}
	req, ok, err := FindProjectVersion(dir)
	if err != nil || !ok || req.Version != "16.10" || req.Source != path {
		t.Fatalf("round trip: %+v ok=%v err=%v", req, ok, err)
	}
}

func TestLatestInstalledMatch(t *testing.T) {
	withTempHome(t)
	dir, err := VersionsDir()
	if err != nil {
		t.Fatal(err)
	}
	// 16.9.0 sorts after 16.16.0 as a string; the match must compare
	// versions numerically.
	for _, v := range []string{"16.9.0", "16.16.0", "17.2.0"} {
		if err := os.MkdirAll(filepath.Join(dir, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]string{"16": "16.16.0", "16.9": "16.9.0", "17": "17.2.0", "17.2.0": "17.2.0"}
	for in, want := range cases {
		got, ok, err := LatestInstalledMatch(in)
		if err != nil || !ok || got != want {
			t.Errorf("LatestInstalledMatch(%q) = %q, %v, %v; want %q", in, got, ok, err, want)
		}
	}
	if _, ok, err := LatestInstalledMatch("15"); err != nil || ok {
		t.Errorf("LatestInstalledMatch(15) ok=%v err=%v, want no match", ok, err)
	}
}

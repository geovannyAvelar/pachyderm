package postgres

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func buildTar(t *testing.T, entries map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if content == "" && name[len(name)-1] == '/' {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestExtractTarStripsTopLevelDir(t *testing.T) {
	dest := t.TempDir()
	buf := buildTar(t, map[string]string{
		"postgresql-16.14.0-x86_64-unknown-linux-gnu/":          "",
		"postgresql-16.14.0-x86_64-unknown-linux-gnu/bin/pg":    "binary",
		"postgresql-16.14.0-x86_64-unknown-linux-gnu/README.md": "hi",
	})

	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "bin", "pg"))
	if err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	if string(data) != "binary" {
		t.Fatalf("got %q, want %q", data, "binary")
	}
}

func TestExtractTarRejectsPathTraversal(t *testing.T) {
	dest := t.TempDir()
	buf := buildTar(t, map[string]string{
		"root/../../evil": "gotcha",
	})

	if err := extractTar(buf, dest); err == nil {
		t.Fatal("expected error for path traversal entry")
	}
}

func TestInstallLifecycle(t *testing.T) {
	withTempHome(t)

	installed, err := IsInstalled("16.14.0")
	if err != nil {
		t.Fatalf("IsInstalled: %v", err)
	}
	if installed {
		t.Fatal("expected version to not be installed yet")
	}

	dir, err := InstallDir("16.14.0")
	if err != nil {
		t.Fatalf("InstallDir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatalf("seeding install dir: %v", err)
	}

	installed, err = IsInstalled("16.14.0")
	if err != nil {
		t.Fatalf("IsInstalled: %v", err)
	}
	if !installed {
		t.Fatal("expected version to be installed")
	}

	versions, err := ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(versions) != 1 || versions[0] != "16.14.0" {
		t.Fatalf("ListInstalled = %v, want [16.14.0]", versions)
	}

	if err := SetCurrent("16.14.0"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	current, err := CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if current != "16.14.0" {
		t.Fatalf("CurrentVersion = %q, want 16.14.0", current)
	}

	if err := Uninstall("16.14.0"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	installed, err = IsInstalled("16.14.0")
	if err != nil {
		t.Fatalf("IsInstalled: %v", err)
	}
	if installed {
		t.Fatal("expected version to be removed after uninstall")
	}

	current, err = CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if current != "" {
		t.Fatalf("expected current symlink to be cleared, got %q", current)
	}
}

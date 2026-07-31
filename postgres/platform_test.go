package postgres

import "testing"

func TestTargetFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"darwin", "arm64", "aarch64-apple-darwin"},
		{"darwin", "amd64", "x86_64-apple-darwin"},
		{"linux", "amd64", "x86_64-unknown-linux-gnu"},
		{"linux", "arm64", "aarch64-unknown-linux-gnu"},
		{"windows", "amd64", "x86_64-pc-windows-msvc"},
	}

	for _, c := range cases {
		got, err := TargetFor(c.goos, c.goarch)
		if err != nil {
			t.Errorf("TargetFor(%s, %s): unexpected error: %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want {
			t.Errorf("TargetFor(%s, %s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestTargetForUnsupported(t *testing.T) {
	if _, err := TargetFor("plan9", "amd64"); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

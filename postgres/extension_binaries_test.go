package postgres

import "testing"

func TestSelectPostGISTag(t *testing.T) {
	tags := []string{"v3.5.7", "v3.5.6", "v3.4.5"}

	cases := []struct {
		version string
		want    string
	}{
		{"3.5.7", "v3.5.7"},
		{"3.5", "v3.5.7"},
		{"3", "v3.5.7"},
		{"3.4", "v3.4.5"},
	}

	for _, c := range cases {
		got, err := SelectPostGISTag(tags, c.version)
		if err != nil {
			t.Errorf("SelectPostGISTag(%q): unexpected error: %v", c.version, err)
			continue
		}
		if got != c.want {
			t.Errorf("SelectPostGISTag(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}

func TestSelectPostGISTagNotFound(t *testing.T) {
	tags := []string{"v3.5.7"}

	if _, err := SelectPostGISTag(tags, "2.5"); err == nil {
		t.Fatal("expected error for version with no matching release")
	}
}

func TestPostGISAssetURL(t *testing.T) {
	got := PostGISAssetURL("v3.5.7", "17", "x86_64-unknown-linux-gnu")
	want := "https://github.com/geovannyAvelar/postgis-binaries/releases/download/v3.5.7/postgis-3.5.7-pg17-x86_64-unknown-linux-gnu.tar.gz"
	if got != want {
		t.Errorf("PostGISAssetURL() = %q, want %q", got, want)
	}
}

func TestInstallPostGISBinaryRequiresInstalledVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := InstallPostGISBinary("17.11.0", "3.5"); err == nil {
		t.Fatal("expected error for a PostgreSQL version that isn't installed")
	}
}

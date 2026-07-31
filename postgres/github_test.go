package postgres

import "testing"

func TestSelectTag(t *testing.T) {
	tags := []string{
		"18.4.0", "17.10.0", "16.14.0", "16.13.0", "16.4.1", "16.4.0",
		"16.2.3", "16.2.1", "15.18.0", "13.23.0",
	}

	cases := []struct {
		version string
		want    string
	}{
		{"16", "16.14.0"},      // latest minor+build for a major
		{"16.14", "16.14.0"},   // exact minor, single build
		{"16.4", "16.4.1"},     // exact minor, picks latest rebuild
		{"16.2", "16.2.3"},     // exact minor, picks latest rebuild
		{"16.14.0", "16.14.0"}, // exact tag
		{"13", "13.23.0"},
	}

	for _, c := range cases {
		got, err := SelectTag(tags, c.version)
		if err != nil {
			t.Errorf("SelectTag(%q): unexpected error: %v", c.version, err)
			continue
		}
		if got != c.want {
			t.Errorf("SelectTag(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}

func TestSelectTagNotFound(t *testing.T) {
	tags := []string{"16.14.0", "15.18.0"}

	if _, err := SelectTag(tags, "9.6"); err == nil {
		t.Fatal("expected error for version with no matching release")
	}
}

func TestSelectTagIgnoresMalformedTags(t *testing.T) {
	tags := []string{"not-a-version", "16.14.0"}

	got, err := SelectTag(tags, "16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "16.14.0" {
		t.Fatalf("got %q, want 16.14.0", got)
	}
}

package postgres

import (
	"strings"
	"testing"
)

func TestGetVersions(t *testing.T) {
	versions, err := GetVersions("versions.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(versions) == 0 {
		t.Fatal("expected non-empty version list")
	}
}

// TestEmbeddedCatalogIsConsistent guards the hand-maintained versions.json
// against editing mistakes.
func TestEmbeddedCatalogIsConsistent(t *testing.T) {
	versions, err := GetVersions("")
	if err != nil {
		t.Fatalf("GetVersions: %v", err)
	}

	seen := map[string]bool{}
	var prev []int
	for i, v := range versions {
		if seen[v.Version] {
			t.Errorf("major version %s is listed twice", v.Version)
		}
		seen[v.Version] = true

		if MajorVersion(v.Version) != v.Version {
			t.Errorf("%q is not a major version", v.Version)
		}
		if !strings.HasPrefix(v.CurrentMinor, v.Version+".") {
			t.Errorf("current minor %q doesn't belong to major version %s", v.CurrentMinor, v.Version)
		}

		parts, err := parseVersionParts(v.Version)
		if err != nil {
			t.Errorf("major version %q: %v", v.Version, err)
			continue
		}
		if i > 0 && compareParts(parts, prev) >= 0 {
			t.Errorf("major version %s is out of order: the catalog lists the newest first", v.Version)
		}
		prev = parts

		// PostgreSQL supports a major version for five years, so support
		// can only end oldest first: a supported version can't follow an
		// unsupported one.
		if i > 0 && v.Supported && !versions[i-1].Supported {
			t.Errorf("major version %s is supported, but the newer %s isn't", v.Version, versions[i-1].Version)
		}
	}

	if !versions[0].Supported {
		t.Errorf("the newest major version, %s, should be supported", versions[0].Version)
	}
}

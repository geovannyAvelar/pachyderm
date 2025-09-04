package postgres

import (
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

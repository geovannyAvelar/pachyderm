package postgres

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

//go:embed versions.json
var embeddedVersions []byte

type Version struct {
	Version      string `json:"version"`
	CurrentMinor string `json:"current_minor"`
	Supported    bool   `json:"supported"`
}

// GetVersions returns the PostgreSQL major-version catalog. An empty source
// uses the catalog embedded in the binary; otherwise source may be a file
// path or an http(s) URL to fetch an updated catalog from.
func GetVersions(source string) ([]Version, error) {
	var content []byte

	if source == "" {
		content = embeddedVersions
	} else if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch versions: %s", resp.Status)
		}

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		file, err := os.Open(source)
		if err != nil {
			return nil, err
		}

		defer file.Close()
		content, err = io.ReadAll(file)
		if err != nil {
			return nil, err
		}
	}

	var versions []Version
	if err := json.Unmarshal(content, &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

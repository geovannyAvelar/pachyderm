package postgres

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Version struct {
	Version      string `json:"version"`
	CurrentMinor string `json:"current_minor"`
	Supported    bool   `json:"supported"`
}

func (v Version) GetUrl() string {
	if v.Supported {
		return fmt.Sprintf("https://get.enterprisedb.com/postgresql/postgresql-%s-1-windows-x64-binaries.zip", v.Version)
	}

	return ""
}

func GetVersions(url string) ([]Version, error) {
	var content []byte

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		resp, err := http.Get(url)
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
		file, err := os.Open(url)
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

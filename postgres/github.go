package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// releasesRepo is the source of prebuilt PostgreSQL binaries used by pachyderm.
const releasesRepo = "theseus-rs/postgresql-binaries"

type ghRelease struct {
	TagName string `json:"tag_name"`
}

var linkNextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// FetchReleaseTags returns every release tag published in releasesRepo, newest first.
func FetchReleaseTags() ([]string, error) {
	var tags []string

	client := &http.Client{}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100", releasesRepo)

	for page := 0; url != "" && page < 30; page++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("listing releases: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("listing releases: unexpected status %s", resp.Status)
		}

		var releases []ghRelease
		err = json.NewDecoder(resp.Body).Decode(&releases)
		next := nextPageURL(resp.Header.Get("Link"))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("listing releases: %w", err)
		}

		for _, r := range releases {
			tags = append(tags, r.TagName)
		}

		url = next
	}

	return tags, nil
}

func nextPageURL(linkHeader string) string {
	if m := linkNextRe.FindStringSubmatch(linkHeader); m != nil {
		return m[1]
	}
	return ""
}

// SelectTag returns the best release tag matching version, which may be a
// major version ("16"), a PostgreSQL minor version ("16.10"), or an exact
// release tag ("16.10.0"). When more than one release matches, the one with
// the greatest trailing version components wins (e.g. the latest minor and
// the latest rebuild of that minor).
func SelectTag(tags []string, version string) (string, error) {
	want, err := parseVersionParts(version)
	if err != nil {
		return "", err
	}

	var best string
	var bestParts []int
	for _, tag := range tags {
		parts, err := parseVersionParts(tag)
		if err != nil || len(parts) < len(want) || !prefixEqual(parts, want) {
			continue
		}

		if best == "" || compareParts(parts, bestParts) > 0 {
			best = tag
			bestParts = parts
		}
	}

	if best == "" {
		return "", fmt.Errorf("no prebuilt PostgreSQL binaries found for version %q", version)
	}

	return best, nil
}

func parseVersionParts(v string) ([]int, error) {
	fields := strings.Split(v, ".")
	parts := make([]int, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("invalid version component %q in %q", f, v)
		}
		parts[i] = n
	}
	return parts, nil
}

func prefixEqual(parts, prefix []int) bool {
	for i, p := range prefix {
		if parts[i] != p {
			return false
		}
	}
	return true
}

func compareParts(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return len(a) - len(b)
}

// AssetURL builds the download URL for a release tag and target triple.
func AssetURL(tag, target string) string {
	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/postgresql-%s-%s.tar.gz",
		releasesRepo, tag, tag, target,
	)
}

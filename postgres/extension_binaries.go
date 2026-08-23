package postgres

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
)

// postgisBinariesRepo is the source of prebuilt PostGIS binaries, built
// against theseus-rs/postgresql-binaries releases. See
// https://github.com/geovannyAvelar/postgis-binaries.
const postgisBinariesRepo = "geovannyAvelar/postgis-binaries"

// FetchPostGISReleaseTags returns every PostGIS release tag published in
// postgisBinariesRepo, newest first.
func FetchPostGISReleaseTags() ([]string, error) {
	return fetchReleaseTags(postgisBinariesRepo)
}

// SelectPostGISTag returns the best release tag matching version (a major,
// minor, or exact PostGIS version, as accepted by SelectTag). Unlike
// postgresql-binaries, postgis-binaries tags carry a leading "v"
// (e.g. "v3.5.7"), so that's stripped before the shared version-matching
// logic runs.
func SelectPostGISTag(tags []string, version string) (string, error) {
	stripped := make([]string, len(tags))
	unprefixed := make(map[string]string, len(tags))
	for i, t := range tags {
		s := strings.TrimPrefix(t, "v")
		stripped[i] = s
		unprefixed[s] = t
	}

	best, err := SelectTag(stripped, version)
	if err != nil {
		return "", fmt.Errorf("no prebuilt PostGIS binaries found for version %q", version)
	}
	return unprefixed[best], nil
}

// PostGISAssetURL builds the download URL for a PostGIS release tag, target
// PostgreSQL major version (e.g. "17"), and target triple.
func PostGISAssetURL(tag, pgMajor, target string) string {
	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/postgis-%s-pg%s-%s.tar.gz",
		postgisBinariesRepo, tag, strings.TrimPrefix(tag, "v"), pgMajor, target,
	)
}

// InstallPostGISBinary downloads a prebuilt PostGIS archive matching
// postgisVersion and overlays it onto an already-installed PostgreSQL
// version's directory, so the extension becomes visible to ListExtensions
// and installable via InstallExtension. pgVersion must already be
// installed (see IsInstalled) and is expected to be a full theseus-rs
// release tag (e.g. "17.11.0"), from which the PostgreSQL major version is
// derived.
func InstallPostGISBinary(pgVersion, postgisVersion string) error {
	installed, err := IsInstalled(pgVersion)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("PostgreSQL %s is not installed", pgVersion)
	}

	pgMajor, _, found := strings.Cut(pgVersion, ".")
	if !found {
		return fmt.Errorf("invalid PostgreSQL version %q", pgVersion)
	}

	target, err := Target()
	if err != nil {
		return err
	}

	tags, err := FetchPostGISReleaseTags()
	if err != nil {
		return err
	}

	tag, err := SelectPostGISTag(tags, postgisVersion)
	if err != nil {
		return err
	}

	dir, err := InstallDir(pgVersion)
	if err != nil {
		return err
	}

	url := PostGISAssetURL(tag, pgMajor, target)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no PostGIS %s binaries published for PostgreSQL %s (%s)", tag, pgMajor, target)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}
	defer gz.Close()

	if err := extractTar(gz, dir); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	return nil
}

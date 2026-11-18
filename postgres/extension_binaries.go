package postgres

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// postgisBinariesRepo is the source of prebuilt PostGIS binaries, built
// against theseus-rs/postgresql-binaries releases. See
// https://github.com/geovannyAvelar/postgis-binaries.
const postgisBinariesRepo = "geovannyAvelar/postgis-binaries"

// DefaultPostGISVersion is the PostGIS version series pachyderm currently
// publishes binaries for.
const DefaultPostGISVersion = "3.5"

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
		"%s/%s/releases/download/%s/%s",
		githubDownloadBase, postgisBinariesRepo, tag, postGISAssetName(tag, pgMajor, target),
	)
}

func postGISAssetName(tag, pgMajor, target string) string {
	return fmt.Sprintf("postgis-%s-pg%s-%s.tar.gz", strings.TrimPrefix(tag, "v"), pgMajor, target)
}

// postGISAssetSHA256 returns the expected SHA-256 digest of a release's
// asset: the digest GitHub computes for every uploaded asset, or else a
// "<asset>.sha256" file published next to it.
func postGISAssetSHA256(release ghRelease, assetName, assetURL string) (string, error) {
	hasChecksumFile := false
	for _, a := range release.Assets {
		if a.Name == assetName && a.Digest != "" {
			return parseGitHubDigest(a.Digest)
		}
		if a.Name == assetName+".sha256" {
			hasChecksumFile = true
		}
	}
	if hasChecksumFile {
		return fetchSHA256File(assetURL + ".sha256")
	}
	return "", fmt.Errorf("no checksum published for %s", assetName)
}

// InstallPostGISBinary downloads a prebuilt PostGIS archive matching
// postgisVersion, checks it against its published SHA-256 checksum, and
// overlays it onto an already-installed PostgreSQL version's directory, so
// the extension becomes visible to ListExtensions and installable via
// InstallExtension. pgVersion must already be installed (see IsInstalled)
// and is expected to be a full theseus-rs release tag (e.g. "17.11.0"),
// from which the PostgreSQL major version is derived.
//
// The archive is verified and extracted into a staging directory before
// anything is moved into the PostgreSQL install, so a failed or corrupted
// download leaves that install as it was.
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

	releases, err := fetchReleases(postgisBinariesRepo)
	if err != nil {
		return err
	}
	tags := make([]string, len(releases))
	for i, r := range releases {
		tags[i] = r.TagName
	}

	tag, err := SelectPostGISTag(tags, postgisVersion)
	if err != nil {
		return err
	}

	var release ghRelease
	for _, r := range releases {
		if r.TagName == tag {
			release = r
			break
		}
	}

	assetName := postGISAssetName(tag, pgMajor, target)
	found = false
	for _, a := range release.Assets {
		if a.Name == assetName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no PostGIS %s binaries published for PostgreSQL %s (%s)", tag, pgMajor, target)
	}

	url := PostGISAssetURL(tag, pgMajor, target)
	want, err := postGISAssetSHA256(release, assetName, url)
	if err != nil {
		return fmt.Errorf("fetching checksum: %w", err)
	}

	dir, err := InstallDir(pgVersion)
	if err != nil {
		return err
	}
	versionsDir := filepath.Dir(dir)
	cleanStaleTemp(versionsDir)

	archive, err := downloadVerified(url, versionsDir, want)
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()

	staging, err := os.MkdirTemp(versionsDir, stagingTempPrefix+"postgis-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := extractTarGz(archive, staging); err != nil {
		return err
	}
	if err := mergeDir(staging, dir); err != nil {
		return fmt.Errorf("installing PostGIS into PostgreSQL %s: %w", pgVersion, err)
	}

	return nil
}

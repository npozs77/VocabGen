// Package update provides shared update-checking logic for both the web UI and CLI.
// It queries the GitHub Releases API, compares semver versions, and builds
// download URLs and delta changelogs.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const (
	// GithubReleasesURL is the GitHub Releases API endpoint for VocabGen.
	GithubReleasesURL = "https://api.github.com/repos/npozs77/VocabGen/releases"
	// DownloadURLBase is the base URL for goreleaser archive downloads.
	DownloadURLBase = "https://github.com/npozs77/VocabGen/releases/download"
)

// UpdateInfo holds the result of a GitHub Releases API check.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	DownloadURL    string
	ChangelogHTML  template.HTML // For web UI (rendered markdown)
	ChangelogText  string        // For CLI (plain text output)
	CheckedAt      string        // ISO 8601 timestamp of the check
	Error          string        // Non-empty if check failed
}

// GithubRelease represents a single release from the GitHub API.
type GithubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

// ParseSemver extracts major, minor, patch from a version string like "1.0.4" or "v1.0.4".
// It also handles git-describe output like "v1.0.3-1-g87fe3a1-dirty" by stripping the
// pre-release suffix (everything after the first "-" in the patch component).
func ParseSemver(v string) (major, minor, patch int, err error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid semver: %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semver major: %q", v)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semver minor: %q", v)
	}
	// Strip pre-release suffix (e.g., "3-1-g87fe3a1-dirty" → "3").
	patchStr := parts[2]
	if idx := strings.Index(patchStr, "-"); idx != -1 {
		patchStr = patchStr[:idx]
	}
	patch, err = strconv.Atoi(patchStr)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semver patch: %q", v)
	}
	return major, minor, patch, nil
}

// IsNewer returns true if version a is newer than version b in semver ordering.
func IsNewer(a, b string) bool {
	aMaj, aMin, aPat, err := ParseSemver(a)
	if err != nil {
		return false
	}
	bMaj, bMin, bPat, err := ParseSemver(b)
	if err != nil {
		return false
	}
	if aMaj != bMaj {
		return aMaj > bMaj
	}
	if aMin != bMin {
		return aMin > bMin
	}
	return aPat > bPat
}

// BuildDownloadURL constructs the goreleaser archive URL for the given version and OS/arch.
func BuildDownloadURL(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s/v%s/vocabgen_%s_%s.%s", DownloadURLBase, version, goos, goarch, ext)
}

// BuildDeltaChangelog renders the release bodies of all newer versions as HTML.
func BuildDeltaChangelog(releases []GithubRelease) template.HTML {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	var out bytes.Buffer
	for _, rel := range releases {
		_, _ = fmt.Fprintf(&out, "<h3>%s</h3>\n", template.HTMLEscapeString(rel.TagName))
		if rel.Body != "" {
			var buf bytes.Buffer
			if err := md.Convert([]byte(rel.Body), &buf); err == nil {
				_, _ = out.Write(buf.Bytes())
			}
		}
	}
	return template.HTML(out.String())
}

// BuildDeltaChangelogText produces a plain text changelog for CLI use.
// Each release is shown with a version header followed by the raw body text.
func BuildDeltaChangelogText(releases []GithubRelease) string {
	var out strings.Builder
	for i, rel := range releases {
		if i > 0 {
			_, _ = out.WriteString("\n")
		}
		_, _ = fmt.Fprintf(&out, "=== %s ===\n", rel.TagName)
		if rel.Body != "" {
			_, _ = out.WriteString(rel.Body)
			if !strings.HasSuffix(rel.Body, "\n") {
				_, _ = out.WriteString("\n")
			}
		}
	}
	return out.String()
}

// CheckNow queries the GitHub Releases API and returns update info.
// It fetches releases, filters those newer than currentVersion, sorts by
// version descending, builds download URL using runtime.GOOS/GOARCH,
// and builds both HTML and plain text delta changelogs.
func CheckNow(ctx context.Context, currentVersion string) *UpdateInfo {
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	// Check if current version is valid semver. Non-release builds (e.g. "dev")
	// still proceed — they compare against all releases and show the latest.
	_, _, _, semverErr := ParseSemver(currentVersion)

	info := &UpdateInfo{
		CurrentVersion: currentVersion,
	}

	releases, err := fetchReleases(ctx)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	if semverErr != nil {
		// Non-semver version (e.g. "dev"): show the latest release as an update.
		return latestAsUpdate(info, releases)
	}

	// Filter releases newer than current version.
	var newer []GithubRelease
	for _, rel := range releases {
		tag := strings.TrimPrefix(rel.TagName, "v")
		if IsNewer(tag, currentVersion) {
			newer = append(newer, rel)
		}
	}

	if len(newer) == 0 {
		info.LatestVersion = currentVersion
		return info
	}

	sortReleasesDesc(newer)

	latest := strings.TrimPrefix(newer[0].TagName, "v")
	info.LatestVersion = latest
	info.HasUpdate = true
	info.DownloadURL = BuildDownloadURL(latest, runtime.GOOS, runtime.GOARCH)
	info.ChangelogHTML = BuildDeltaChangelog(newer)
	info.ChangelogText = BuildDeltaChangelogText(newer)

	return info
}

// fetchReleases queries the GitHub Releases API and returns the list of releases.
func fetchReleases(ctx context.Context) ([]GithubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GithubReleasesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	return releases, nil
}

// sortReleasesDesc sorts releases by semver descending (latest first).
func sortReleasesDesc(releases []GithubRelease) {
	sort.Slice(releases, func(i, j int) bool {
		return IsNewer(
			strings.TrimPrefix(releases[i].TagName, "v"),
			strings.TrimPrefix(releases[j].TagName, "v"),
		)
	})
}

// latestAsUpdate finds the highest-versioned release and returns it as an update.
// Used for non-semver current versions (e.g. "dev") where all releases are candidates.
func latestAsUpdate(info *UpdateInfo, releases []GithubRelease) *UpdateInfo {
	// Filter to only valid semver releases.
	var valid []GithubRelease
	for _, rel := range releases {
		if _, _, _, err := ParseSemver(rel.TagName); err == nil {
			valid = append(valid, rel)
		}
	}
	if len(valid) == 0 {
		info.LatestVersion = info.CurrentVersion
		return info
	}

	sortReleasesDesc(valid)

	latest := strings.TrimPrefix(valid[0].TagName, "v")
	info.LatestVersion = latest
	info.HasUpdate = true
	info.DownloadURL = BuildDownloadURL(latest, runtime.GOOS, runtime.GOARCH)
	info.ChangelogHTML = BuildDeltaChangelog(valid)
	info.ChangelogText = BuildDeltaChangelogText(valid)
	return info
}

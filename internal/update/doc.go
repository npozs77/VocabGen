// Package update provides shared update-checking logic for both the web UI and
// CLI. It queries the GitHub Releases API, compares semver versions, builds
// OS/arch-aware download URLs, and renders delta changelogs.
package update

#!/usr/bin/env bash
# Release checklist script. Ensures quality gates pass before tagging.
# Usage:
#   ./scripts/release.sh v1.0.0
#   ./scripts/release.sh -y v1.0.0   # skip confirmation prompt

set -euo pipefail

AUTO_YES=false
if [ "${1:-}" = "-y" ]; then
    AUTO_YES=true
    shift
fi

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo "Usage: ./scripts/release.sh [-y] <version>"
    echo "Example: ./scripts/release.sh v1.0.0"
    echo "  -y  Skip confirmation prompt (for CI/automation)"
    exit 1
fi

if ! echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "ERROR: Version must match vMAJOR.MINOR.PATCH (e.g., v1.0.0)"
    exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
    echo "ERROR: Working directory is not clean. Commit or stash changes first."
    exit 1
fi

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
    echo "Currently on $BRANCH, switching to main..."
    git checkout main || { echo "ERROR: Failed to switch to main."; exit 1; }
fi

echo "--- Pulling latest main ---"
git pull origin main --ff-only || { echo "ERROR: Failed to fast-forward main. Resolve manually."; exit 1; }

echo "=== Running quality checks ==="

echo "--- Build ---"
make build

echo "--- Vet ---"
make vet

echo "--- Format Check ---"
make fmt-check

echo "--- Tests ---"
make test

echo ""
echo "=== All checks passed ==="
echo ""
echo "Ready to release $VERSION"
echo "This will:"
echo "  1. Create git tag $VERSION"
echo "  2. Push the tag to origin"
echo "  3. GitHub Actions + goreleaser will build and publish the release"
echo ""
if [ "$AUTO_YES" = true ]; then
    REPLY=y
else
    read -p "Proceed? (y/N) " -n 1 -r
    echo ""
fi

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

echo ""
echo "Tag $VERSION pushed. GitHub Actions will handle the rest."
echo "Watch the release at: https://github.com/user/vocabgen/actions"

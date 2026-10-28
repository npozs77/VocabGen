---
name: release
description: Cut a VocabGen release — validate commits since the last tag, confirm the CHANGELOG entry and version, then tag via scripts/release.sh.
disable-model-invocation: true
---

# Release

Stop at any failed check and tell the user why. Don't edit `CHANGELOG.md` here — a missing entry goes in via a normal PR first.

1. **Sync main.** If not on `main`, say so, then `git checkout main && git pull origin main`.
2. **Latest tag.** `git tag --list 'v*' --sort=-v:refname | head -1`.
3. **What's shipping.** `git log <tag>..origin/main --oneline` and `--merges --oneline`. Report "Since <tag>: X commits, Y pull requests". Zero commits → nothing to release, stop.
4. **Changelog preview.** Find the first `## [X.Y.Z]` heading in `CHANGELOG.md` newer than the latest tag and show the whole section. None → "Add the changelog entry in a PR first", stop. Ask whether it looks right.
5. **Version.** Suggest `vX.Y.Z` from that heading; if the log has `feat:` commits, confirm it is at least a minor bump. Ask for the tag to use and verify `## [<version without v>]` exists in `CHANGELOG.md`.
6. **Tag.** Run `./scripts/release.sh -y <tag>` (timeout 300s). It re-runs quality checks, tags and pushes; the tag triggers `.github/workflows/release.yml` (goreleaser + multi-arch images to `ghcr.io/npozs77/vocabgen`).
7. **Report** the tag and the Actions run link (`gh run list --workflow release.yml --limit 1`).

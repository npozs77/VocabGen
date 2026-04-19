# Deployment

## Platform Support

| Platform | Architecture | Binary Name |
|----------|-------------|-------------|
| macOS | amd64 | `vocabgen_darwin_amd64` |
| macOS | arm64 (Apple Silicon) | `vocabgen_darwin_arm64` |
| Linux | amd64 | `vocabgen_linux_amd64` |
| Linux | arm64 | `vocabgen_linux_arm64` |
| Windows | amd64 | `vocabgen_windows_amd64.exe` |

## Cross-Compilation with goreleaser

### goreleaser Configuration

Create `.goreleaser.yaml` at the repo root:

```yaml
version: 2
project_name: vocabgen
builds:
  - main: ./cmd/vocabgen
    binary: vocabgen
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    exclude:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.buildDate={{.Date}}
archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
checksum:
  name_template: checksums.txt
changelog:
  sort: asc
```

### Local Snapshot Build

```bash
goreleaser release --snapshot --clean
```

Produces binaries in `dist/` without publishing.

## GitHub Actions CI

Create `.github/workflows/ci.yml`:

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make build
      - run: make vet
      - run: make test
```

### Release Workflow

Create `.github/workflows/release.yml`:

```yaml
name: Release
on:
  push:
    tags:
      - "v*"

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Release Process

Changelog is updated as part of each feature/fix PR — not as a separate step. Each PR adds its entry under the target version heading in `CHANGELOG.md` (e.g., `## [1.0.3] - 2026-04-04`).

When ready to release:

```bash
./scripts/release.sh v1.0.3
```

The script:
1. Verifies `CHANGELOG.md` has an entry for the version
2. Checks clean working directory, switches to `main`, pulls latest
3. Runs build + vet + fmt-check + tests
4. Tags and pushes — GitHub Actions + goreleaser handle the rest

## Versioning

Semantic versioning: `vMAJOR.MINOR.PATCH`

Version is injected at build time via ldflags:

```bash
go build -ldflags "-X main.version=v1.0.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o vocabgen ./cmd/vocabgen
```

## IDE Setup (VS Code / Kiro)

Install the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go) (`golang.Go`). It provides:
- `gopls` language server (auto-completion, go-to-definition, rename, diagnostics)
- `goimports` format-on-save (organizes imports + formats)
- Package-level lint-on-save via `golangci-lint`
- Test explorer integration (`go test` from the sidebar)
- Debugger via Delve (`dlv`)

Recommended `.vscode/settings.json` (already committed):

```json
{
  "go.useLanguageServer": true,
  "editor.formatOnSave": true,
  "go.formatTool": "goimports",
  "go.lintOnSave": "package"
}
```

This means `golangci-lint` runs on every save at the package level — catching issues from the default linters (errcheck, govet, staticcheck) plus extended linters configured in `.golangci.yml` (errorlint, gocritic, revive, bodyclose, noctx, godoclint) before you even run `make quality`.

## Upgrading

vocabgen can check for newer versions via the GitHub Releases API. User data in `~/.vocabgen/` (config and database) is independent of the binary and unaffected by upgrades.

### Check for Updates

From the CLI:

```bash
vocabgen update
```

Displays current version, latest version, a platform-specific download link, and a delta changelog. `vocabgen version` also appends a one-line notice when a newer version exists.

From the web UI: navigate to Help → Check for Update (`/update`). The page queries GitHub Releases on load and shows the same information. A dismissible banner also appears on all pages when an update is detected at server startup.

### Apply an Update

Download the new binary for your platform and replace the existing one:

```bash
# macOS (Apple Silicon) example
curl -LO https://github.com/npozs77/VocabGen/releases/latest/download/vocabgen_darwin_arm64.tar.gz
tar xzf vocabgen_darwin_arm64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/
```

No migration steps needed — `~/.vocabgen/config.yaml` and `~/.vocabgen/vocabgen.db` are preserved across binary replacements.

## Docker (Optional)

Minimal `FROM scratch` image since vocabgen is a static binary:

```dockerfile
FROM scratch
COPY vocabgen /vocabgen
VOLUME /root/.vocabgen
EXPOSE 8080
ENTRYPOINT ["/vocabgen"]
```

Build and run:

```bash
CGO_ENABLED=0 go build -o vocabgen ./cmd/vocabgen
docker build -t vocabgen .
docker run -v ~/.vocabgen:/root/.vocabgen -p 8080:8080 vocabgen serve
```

The volume mount at `/root/.vocabgen` persists the config file and SQLite database between container restarts.

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
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.buildDate={{.Date}}
archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: checksums.txt
changelog:
  disable: true
dockers:
  - image_templates:
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-amd64"
    use: buildx
    dockerfile: Dockerfile
    build_flag_templates:
      - "--platform=linux/amd64"
    goarch: amd64
    goos: linux
  - image_templates:
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-arm64"
    use: buildx
    dockerfile: Dockerfile
    build_flag_templates:
      - "--platform=linux/arm64"
    goarch: arm64
    goos: linux
docker_manifests:
  - name_template: "ghcr.io/npozs77/vocabgen:{{ .Version }}"
    image_templates:
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-amd64"
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-arm64"
  - name_template: "ghcr.io/npozs77/vocabgen:latest"
    image_templates:
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-amd64"
      - "ghcr.io/npozs77/vocabgen:{{ .Version }}-arm64"
```

### Docker Images

goreleaser also builds multi-arch Docker images (amd64/arm64) and pushes them to `ghcr.io/npozs77/vocabgen`. The `dockers` section defines per-arch builds using buildx, and `docker_manifests` creates versioned + `latest` multi-arch manifests.

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
      packages: write
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3
      - uses: goreleaser/goreleaser-action@v7
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

## Development Workflow

### Build Targets

| Target | Output | Purpose |
|--------|--------|---------|
| `make build` | `./vocabgen` | Production build (CI, releases) |
| `make dev` | `bin/vocabgen` | Dev build (feature branches) |
| `make dev-serve` | — | Dev build + launch web server on port 8081 |

### Dev Database

`make dev-serve` passes `--db-path ~/.vocabgen/vocabgen-dev.db` so development testing never touches the production database (`~/.vocabgen/vocabgen.db`). The active database path is displayed in the web UI nav bar next to the profile indicator, making it immediately clear which database you're looking at.

To use a custom database path with the production binary:

```bash
vocabgen serve --port 8080 --db-path ~/.vocabgen/my-project.db
```

The `--db-path` flag overrides the `db_path` setting in `config.yaml`. If the file doesn't exist, it's created automatically with all migrations applied.

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

## Docker

Multi-arch Docker images (amd64/arm64) are published to GitHub Container Registry on each release.

### Quick Start

```bash
docker run -d \
  -p 8080:8080 \
  -v ./data:/data \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  ghcr.io/npozs77/vocabgen:latest
```

Inside the container, vocabgen automatically detects the Docker environment and defaults to `/data/vocabgen.db` for the database (instead of `~/.vocabgen/vocabgen.db`). A simple `-v ./data:/data` mount is all you need — no `chown` or UID mapping required.

### Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent release |
| `1.2.2` | Specific version |

### Volume Mount

Mount `/data` to persist `config.yaml` and `vocabgen.db` across container restarts:

```bash
-v ./data:/data
```

The container runs as a non-root user (UID 65532). The `/data` directory is writable by this user out of the box.

### CLI Commands via Docker

```bash
docker run ghcr.io/npozs77/vocabgen:latest version
docker run -e OPENAI_API_KEY=sk-... ghcr.io/npozs77/vocabgen:latest lookup "werk" -l nl --provider openai
docker run -v ./data:/data ghcr.io/npozs77/vocabgen:latest batch --input-file /data/words.csv --mode words -l nl
```

### Build Locally

The Dockerfile expects a pre-built `vocabgen` binary in the build context (goreleaser provides this during releases). To build locally:

```bash
CGO_ENABLED=0 GOOS=linux go build -o vocabgen ./cmd/vocabgen
docker build -t vocabgen:local .
docker run -p 8080:8080 -v ./data:/data vocabgen:local
```

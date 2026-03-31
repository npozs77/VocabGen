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

Use the release script to enforce all quality gates before tagging:

```bash
./scripts/release.sh v1.0.0
```

The script verifies: clean working directory, on `main` branch, then runs build + vet + fmt-check + tests. If everything passes, it tags and pushes. GitHub Actions + goreleaser handle the rest.

Manual steps (if not using the script):

1. Ensure `main` is green: `make quality`
2. Tag a release: `git tag -a v1.0.0 -m "Release v1.0.0" && git push origin v1.0.0`
3. GitHub Actions triggers

## Versioning

Semantic versioning: `vMAJOR.MINOR.PATCH`

Version is injected at build time via ldflags:

```bash
go build -ldflags "-X main.version=v1.0.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o vocabgen ./cmd/vocabgen
```

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

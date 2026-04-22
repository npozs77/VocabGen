# Changelog

All notable changes to this project will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses [Semantic Versioning](https://semver.org/).

## [1.3.1]

### Changed

- Highlighted provider quality guidance across README, user guide, and marketing page — clearer messaging that paid models produce better results for nuanced vocabulary tasks
- OCI labels on Docker images — GitHub Packages page now shows title, description, version, and license metadata
- Docker packages link added to the marketing page footer
- GitHub Pages deployment now only triggers when `docs/` changes (dedicated workflow with path filter)
- Dependabot auto-rebases open PRs when they fall behind main

## [1.3.0]

### Added

- Flashcards study mode — flip through vocabulary cards, filter by language/tag/difficulty, and rate cards for focused review (#39)

## [1.2.2] - 2026-04-19

### Added

- Nav-bar profile switcher — active config profile visible and switchable from any page (#44)
- Docker image distribution via GitHub Container Registry — `docker run ghcr.io/npozs77/vocabgen` with multi-arch support (amd64/arm64) (#47)

### Fixed

- Enter key no longer submits the batch form when typing in the paste list textarea

## [1.2.1] - 2026-04-18

### Added

- Bulk delete for database entries — select individual entries or use select-all, then delete in one action (#33, #34)
- Batch processing cancellation — cancel a running batch from the Web UI with partial results preserved (#32)

### Fixed

- Config profile switching now correctly refreshes the form and saves to the selected profile (#28)

## [1.2.0] - 2026-04-05

### Added

- Multiple config profiles — save different LLM setups (local, sandbox, prod) and switch via `--profile` or the Web UI (#23)
- One-click local LLM setup — run `scripts/setup-local-llm.sh` or use the Web UI config page to install Ollama and configure a local model (#22)
- E2E tests default to local Ollama — free, offline testing via `--profile local` with pre-flight checks (#24)

## [1.1.0] - 2026-04-04

### Added

- Help dropdown menu in navigation bar replacing standalone About link
- Documentation pages with embedded markdown rendering via goldmark (Architecture, Deployment, User Guide)
- Check for Update page with GitHub Releases API integration, semver comparison, and OS/arch-aware download links
- Delta changelog rendering — shows combined release notes for all versions between current and latest
- Update notification banner on all pages when a newer version is available (dismissible, resets on restart)
- Changelog page rendering embedded CHANGELOG.md as formatted HTML
- `vocabgen update` CLI subcommand — checks GitHub Releases for newer versions, shows download URL and delta changelog
- `vocabgen version` now appends a one-line update notice when a newer version is available
- Shared `internal/update` package for reusable semver parsing, version comparison, and GitHub Releases API client

## [1.0.3] - 2026-04-03

### Added

- Real-time SSE batch streaming with per-item progress events in web UI (#12)

### Fixed

- Expression edit template rendering

## [1.0.2] - 2026-04-03

### Changed

- Web UI config page reads API keys from environment variables instead of form fields, matching CLI behavior
- Provider credential hints shown per provider on config page (env var names, setup instructions)

### Fixed

- Web UI test-connection and batch endpoints no longer require API key in request body
- Provider credential validation on config save — warns if required env vars are missing

## [1.0.0] - 2026-04-02

### Added

- CLI with subcommands: `lookup`, `batch`, `serve`, `backup`, `restore`, `version`
- LLM provider interface with Bedrock, OpenAI, Anthropic, and Vertex AI implementations
- OpenAI-compatible server support (Ollama, LM Studio, Azure, vLLM) via `--base-url`
- SQLite cache layer — checks DB before invoking LLM, eliminates duplicate API calls
- Embedded web UI (HTMX + Tailwind CSS) with Lookup, Batch, Config, Database, and About pages
- Multi-version vocabulary entries with conflict resolution (replace, add, skip)
- Context-aware cache bypass — providing a context sentence forces a fresh LLM lookup
- XLSX export with separate Words and Expressions sheets, respects current filters
- XLSX and CSV import with header detection and column mapping by name
- UTF-8 validation on CSV import — rejects non-UTF-8 files with actionable error message
- Input validation — words containing digits or special characters rejected before LLM call
- Hallucination detection — warns when input token not found in LLM example sentence (words only)
- Non-word detection — warns and skips DB insert when LLM returns type="—" or "not a valid word"
- Warning display in web UI lookup results (yellow banner)
- Database page: type dropdown filter, tags filter with debounce, target translation column
- Full edit form for database entries showing all fields, preserving unchanged fields on save
- Entry tagging via `--tags` flag and web UI
- Batch processing with SSE progress streaming, error resilience, limit enforcement
- Dry-run mode (`--dry-run`) — preview without LLM calls or DB writes
- Config manager with YAML persistence (`~/.vocabgen/config.yaml`), API keys never stored
- Database backup/restore subcommands
- Structured logging via `log/slog` (INFO/DEBUG/ERROR levels, `--verbose` for debug)
- Cross-compilation support via goreleaser (macOS amd64/arm64, Linux amd64/arm64, Windows amd64)

### Supported Languages

Dutch, Hungarian, Italian, Russian, English, German, French, Spanish, Portuguese, Polish, Turkish — plus any language name passed directly via `-l`.

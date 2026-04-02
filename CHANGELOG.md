# Changelog

All notable changes to this project will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses [Semantic Versioning](https://semver.org/).

## [1.0.2] - 2026-04-03

### Changed

- Web UI config page reads API keys from environment variables instead of form fields, matching CLI behavior
- Provider credential hints shown per provider on config page (env var names, setup instructions)

### Fixed

- Web UI test-connection and batch endpoints no longer require API key in request body
- Added provider credential validation on config save — warns if required env vars are missing
- Corrected Bedrock model IDs in docs, added cross-region inference profile note
- Added Windows and macOS Intel installation instructions to user guide

### Docs

- Expanded user guide with Configuration section, Quick Setup via Web UI, CLI flag overrides, and provider credentials table
- README links to Web UI setup instructions

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
- Build-time version injection via `-ldflags`
- GitHub Actions CI: build, vet, fmt-check, staticcheck, tests with race detection
- `workflow_dispatch` for manual CI runs on any branch
- Database helper script (`scripts/db-helper.sh`) for mass updates, tag additions, POS listing
- Property-based tests (P1–P19) with `rapid`, table-driven tests, fuzz tests, integration tests
- Cross-compilation support via goreleaser (macOS amd64/arm64, Linux amd64/arm64, Windows amd64)

### Supported Languages

Dutch, Hungarian, Italian, Russian, English, German, French, Spanish, Portuguese, Polish, Turkish — plus any language name passed directly via `-l`.

# VocabGen Documentation

Welcome to the VocabGen documentation. Use the links below to navigate to the section you need.

---

## [Architecture](/docs/architecture)

System design, package layout, and data flow diagrams.

- [Overview](/docs/architecture#overview) — what vocabgen is and how it works
- [Design Principles](/docs/architecture#design-principles) — single binary, language-agnostic, cache-first
- [System Architecture](/docs/architecture#system-architecture) — component diagram (Mermaid)
- [Package Layout](/docs/architecture#package-layout) — directory structure
- [Package Details](/docs/architecture#package-details) — llm, language, parsing, service, db, output, config, web
- [Data Flow: Single Lookup](/docs/architecture#data-flow-single-lookup) — sequence diagram
- [Data Flow: Batch Processing](/docs/architecture#data-flow-batch-processing) — sequence diagram
- [Data Models](/docs/architecture#data-models) — words and expressions JSON schemas
- [Error Handling](/docs/architecture#error-handling) — error categories and behavior
- [Web UI Architecture](/docs/architecture#web-ui-architecture) — templates, HTMX patterns, API routes
- [Testing Strategy](/docs/architecture#testing-strategy) — PBT + table-driven approach
- [Key Design Decisions](/docs/architecture#key-design-decisions) — technology choices summary

---

## [Deployment](/docs/deployment)

Build, release, and CI/CD configuration.

- [Platform Support](/docs/deployment#platform-support) — supported OS and architecture matrix
- [Cross-Compilation with goreleaser](/docs/deployment#cross-compilation-with-goreleaser) — config and snapshot builds
- [GitHub Actions CI](/docs/deployment#github-actions-ci) — CI and release workflows
- [Release Process](/docs/deployment#release-process) — changelog, tagging, and publishing
- [Versioning](/docs/deployment#versioning) — semver and ldflags injection
- [Docker](/docs/deployment#docker-optional) — optional container setup

---

## [User Guide](/docs/user-guide)

Installation, configuration, and daily usage.

- [Prerequisites](/docs/user-guide#prerequisites) — LLM provider options (paid and free local)
- [Installation](/docs/user-guide#installation) — per-platform download and setup
- [Configuration](/docs/user-guide#configuration) — Web UI, CLI flags, config file
- [Provider Credentials](/docs/user-guide#provider-credentials) — Bedrock, OpenAI, Anthropic, Vertex AI, Ollama
- [Config Profiles](/docs/user-guide#config-profiles) — named profiles, `--profile` flag, Web UI dropdown
- [Local LLM Setup](/docs/user-guide#local-llm-setup) — one-click Ollama setup via script or Web UI
- [First Run](/docs/user-guide#first-run) — your first lookup
- [Batch Processing](/docs/user-guide#batch-processing) — CSV input, limits, dry-run
- [Web UI](/docs/user-guide#web-ui) — Lookup, Batch, Config, Database pages
- [Tags](/docs/user-guide#tags) — tagging entries during lookup and batch
- [Database Management](/docs/user-guide#database-management) — backup, restore, import, export
- [Provider Switching](/docs/user-guide#provider-switching) — Bedrock, OpenAI, Anthropic, Ollama, Azure
- [Dry-Run Mode](/docs/user-guide#dry-run-mode) — preview without API calls
- [Checking for Updates](/docs/user-guide#checking-for-updates) — CLI update check via GitHub Releases
- [User Data and Updates](/docs/user-guide#user-data-and-updates) — where data lives, safe binary upgrades
- [Conflict Resolution](/docs/user-guide#conflict-resolution) — replace, add, skip strategies
- [E2E Testing](/docs/user-guide#e2e-testing) — end-to-end tests with local or cloud profiles
- [Adding Languages](/docs/user-guide#adding-languages) — built-in codes and custom additions

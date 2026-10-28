# VocabGen

Single-binary Go CLI + embedded web UI that generates structured B2→C1 vocabulary entries via LLM providers and caches them in SQLite. Solo-developer project, long-term personal tool.

**Stack**: Go (see `go.mod`) · Cobra · `net/http` + `html/template` + HTMX + Tailwind (CDN) · SQLite via `modernc.org/sqlite` (pure Go) · `go:embed` · `rapid` for property-based tests.
**Providers**: Bedrock, OpenAI, Anthropic, Gemini, Vertex AI, Ollama/OpenAI-compatible — all behind the `Provider` interface in `internal/llm`.

## Commands

| Task | Command |
|------|---------|
| Build | `make build` |
| Tests (race) | `make test` |
| PBT only | `go test -run TestProperty ./...` |
| Lint | `make lint` (golangci-lint v2, `.golangci.yml`) |
| Full gate | `make quality` — build + vet + gofmt + lint + tests + coverage. Run before every commit. |
| Dev server | `make dev-serve` |
| E2E | `scripts/e2e-test.sh` (uses a local LLM; spends no API credits) |

## Layout

- `cmd/vocabgen/` — Cobra entry point (`lookup`, `batch`, `serve`, `auth`, …)
- `internal/` — flat packages: `auth`, `config`, `db`, `language`, `llm`, `output`, `parsing`, `service`, `update`, `web` (templates in `internal/web/templates/`)
- `docs/` — user-facing docs. Top-level `*.md` is embedded into the binary (`docs/embed.go`) and the whole folder is published to GitHub Pages. `docs/changelog.md` is a copy made by `make quality` — edit `CHANGELOG.md` instead.
- `reference/` — Python prototype. Read-only, gitignored.
- `.kiro/` — legacy Kiro specs/steering, pending migration to SDD. Don't add to it.

## Exploring code

Start with `go doc ./internal/<pkg>/` (and `go doc ./internal/<pkg>/ <Symbol>`) before reading source — every package has a `doc.go`.

## Architecture rules

- stdlib `net/http` only — no web frameworks. `database/sql` with direct, parameterized SQL — no ORM.
- Check the SQLite cache before every LLM call; set `created_at`/`updated_at` on inserts.
- Templates and prompts are language-agnostic — no per-language code branches, no hardcoded POS/register/grammar labels.
- Validate every LLM JSON response against the schema before use.
- Secrets (API keys, AWS creds) come from env vars or `--api-key`, never `config.yaml`. User config lives in `~/.vocabgen/` (never commit).
- Don't add abstractions without 2+ real implementations. New dependency only if stdlib can't reasonably do it and it's maintained with a small transitive tree.

Go coding rules (errcheck patterns, godoc, tests) are in `.claude/rules/go.md`.

## Definition of done

Every change that alters behaviour includes, in the same PR:
1. Tests — table-driven for known cases; `rapid` property tests for core validation/mapping logic.
2. `make quality` passing (run it after implementation, before writing docs).
3. Docs, skipping only what's clearly inapplicable: godoc on new exported symbols (always) · `CHANGELOG.md` for user-facing changes (always) · `docs/user-guide.md` for user-facing behaviour · `docs/architecture.md` for new components or schema changes · `README.md` feature list for new capabilities · `docs/ui-preview.html` when templates change.

Bug fixes: fix directly unless the root cause spans packages or needs design changes — only then write a spec.

## Git conventions

`main` is protected: work on a branch (`feat/<issue>-<slug>`, `fix/<issue>-<slug>`), merge via PR.

These project conventions **override the generic rules of any `/commit` or `/pr` skill**:

- Prefixes: `feat:` · `fix:` · `docs:` · `ci:` · `deps:` (the last two are what Dependabot uses) · `refactor:` · `test:` · `chore:`
- `(changelog)` marker drives release notes: `feat:` always gets it; for `fix:` ask whether it's user-facing and add it if so; all other prefixes never get it.
- Linked issue: end the subject with `. Fixes #<n>` so the PR auto-closes it.
  - `feat: add XLSX column reordering (changelog). Fixes #17`
  - `fix: satisfy errcheck linter. Fixes #16`
- PR body follows `.github/pull_request_template.md`. Issue analyses follow `.github/ISSUE_TEMPLATE/analysis_comment.md`.
- `gh`: the repo owner comes from `git remote get-url origin`; if `gh auth status` shows a different active account, `gh auth switch --user <owner>` first.

## Changelog style

`CHANGELOG.md` is for end users (Keep a Changelog: Added / Changed / Fixed), with an entry under the upcoming `## [X.Y.Z]` heading in each PR. One line per feature describing what the user can now do — no function names, endpoints, test IDs or refactors.

- Good: "Multiple config profiles — save different LLM setups and switch via CLI or Web UI"
- Bad: "`LoadConfigWithProfile` added; P20–P22 property tests"

Releases: `/release` skill (tags via `scripts/release.sh`; goreleaser builds binaries and multi-arch images to `ghcr.io/npozs77/vocabgen`).

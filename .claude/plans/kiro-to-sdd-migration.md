# Kiro → SDD migration plan

**Status**: deferred (low priority). Do this before the next feature development, not before.
**Decision**: Strategy A — adopt SDD fully and migrate the legacy Kiro specs with the sdd-toolkit `/migrate` skill. This is also a test of `/migrate` itself: collect feedback for the toolkit as you go (see §6).
**Written**: 2026-09-29, on branch `refactor/migrate`.

## 1. Where things stand

| Item | State |
|------|-------|
| Claude Code setup | Done — commit `db1f9e9` on `refactor/migrate`: `CLAUDE.md`, `.claude/rules/go.md`, gofmt PostToolUse hook, permissions, `/release` skill. Independent of SDD; can merge on its own. |
| App on new machine | Verified 2026-09-29: `make build` OK, `go test -race ./...` all green, `serve` starts, cached lookup reads the DB. |
| Local DBs (`~/.vocabgen/`) | `PRAGMA integrity_check` = ok on all three. `vocabgen.db` 95 words / 27 expressions, `vocabgen-dev.db` 40 words, `test.db` 3/2. WAL mode, no leftover `-wal` files. |
| Legacy specs | Still in `.kiro/`, untouched. |

## 2. Prerequisites (before running anything SDD)

1. **Tooling** — Go 1.27.1 is installed (Homebrew). Still missing, needed by `make quality` and the `/commit` quality gate:
   ```sh
   brew install golangci-lint
   go install golang.org/x/tools/cmd/goimports@latest   # hook falls back to gofmt without it
   ```
2. **Fix local config** — `~/.vocabgen/config.yaml` has `db_path: /Users/norbert.pozsonyi/.vocabgen/test.db` (old machine's home). Point it at `~/.vocabgen/vocabgen.db` (or remove it to use the default).
3. **Back up the legacy archive.** `.kiro/` and `reference/` are gitignored. Only `.kiro/specs/go-vocabulary-generator/` is tracked (force-added); the other five specs, steering, hooks and `roadmap.md` exist on disk only.
   ```sh
   tar czf ~/Archive/vocabgen-kiro-$(date +%F).tgz .kiro reference
   ```
4. **Resolve the `docs/` collision.** SDD writes `docs/state.md`, `docs/audit.log`, `docs/inception/`, `docs/construction/`. In this repo `docs/`:
   - is published wholesale to GitHub Pages (`.github/workflows/pages.yml` uploads `docs/`, triggers on `docs/**`), and
   - has top-level `*.md` embedded in the binary (`docs/embed.go`, `//go:embed *.md`) — would pick up `state.md`.

   Options (pick one before `sdd init`):
   - **a.** Give SDD a different `docsDir` (e.g. `spec/`) — first confirm the toolkit honours a non-`docs/` path everywhere (its constraints hard-code `docs/state.md`).
   - **b.** Move the Pages site to its own folder (e.g. `site/`) and narrow `embed.go` to the four listed files. More churn in app code.
   - **c.** Keep SDD in `docs/`, restrict Pages to an explicit file list, and change the embed pattern to exclude `state.md`.

   Preferred: **a** if the toolkit supports it (zero app changes); otherwise **c**.

## 3. Branching

- Merge the Claude Code setup (`refactor/migrate` @ `db1f9e9` + this plan) as its own PR whenever convenient.
- Do the migration on a fresh branch off `main` (e.g. `refactor/sdd-migrate`) so the SDD artifacts are one reviewable PR.

## 4. Execution

1. **`sdd init`** — brownfield. Expected profile: Go, `artifact_type` = cli (with embedded web server; note if the toolkit can't express "cli + web"), solo, maturity **MVP** (specs have WHEN/SHALL criteria and 19 correctness properties).
2. **`/migrate`** with `specPath: .kiro/specs`. Input inventory:

   | Spec | Type | Lines (req/design/tasks) | Notes |
   |------|------|--------------------------|-------|
   | `go-vocabulary-generator` | feature | 1233 / 1971 / 1065 | Core baseline; req 1–69, properties P1–P19, tasks 1–32 |
   | `multi-user-access` | feature | 475 / 953 / 578 | Auth, users.yaml, API keys |
   | `help-menu` | feature | 249 / 616 / 309 | |
   | `flashcards` | feature | 272 / 336 / 170 | |
   | `sentence-lookup` | feature | 150 / 125 / 96 | |
   | `profile-save-corruption` | bugfix | `bugfix.md` 79 | Kiro bugfix format — no req/design/tasks split |

   Guidance to give the skill:
   - Core spec = baseline requirements; the four feature specs become additional FR areas + units (each already built → units marked done).
   - Preserve correctness properties with their P-numbers — tests are named after them (`TestProperty…`), so traceability depends on it.
   - `profile-save-corruption`: fold root cause + fix into the relevant unit's summary rather than creating a unit.
   - All code is shipped (v1.5.0) → final state should be construction complete / operations, not "inception in progress".
3. **`/review inception`**, then fix flagged gaps.
4. **Spot-check traceability**: pick 3 requirements, confirm FR-ID → unit → code file → test.

## 5. Cleanup after migration

- Delete `.kiro/` (archive already taken in §2.3) and drop `.kiro/` from `.gitignore`.
- Move `.kiro/roadmap.md` ideas into GitHub issues (label `enhancement`); discard the pasted search results.
- `CLAUDE.md`: replace the `.kiro/` layout line with the SDD docs location; add a line that feature work goes through `/inception`→`/construction` and issues through `/issue-analyze`.
- `.github/ISSUE_TEMPLATE/analysis_comment.md`: decide whether it or the toolkit's `/issue-analyze` template wins; delete the loser.

## 6. `/migrate` evaluation — feedback to collect for the toolkit

Record findings here (or as sdd-toolkit issues) while running it:

- [ ] Did it honour the MVP artifact list without asking redundant questions the specs answer?
- [ ] Were all requirements preserved (count FRs vs. old requirement count, 69 core + feature specs)?
- [ ] Were correctness properties kept with stable IDs, or re-numbered/dropped?
- [ ] How did it handle multiple specs (merge into one baseline vs. per-spec phases `docs/0-core/`, `docs/1-…`)?
- [ ] How did it handle the `bugfix.md` format?
- [ ] Unit decomposition vs. real package layout (`internal/*`) — sensible?
- [ ] Contradiction in the skill: Rules say "Generate BLMs for existing code", Anti-patterns say "Creating functional-design BLMs for already-built units". Which did it do?
- [ ] Output paths: skill text lists `docs/inception/user-stories/` in the summary but `requirements/` in the steps.
- [ ] Known gaps in `git-workflow` for this project (currently patched via `CLAUDE.md`):
  - `/commit` lacks the `(changelog)` marker, `Fixes #N`, and `deps:`/`ci:` prefixes (Dependabot).
  - No release skill (project-local `/release` fills it).
- [ ] Token/time cost of the run (8.7k lines of input).

## 7. Done when

- `docs/state.md` (or chosen location) shows the project past construction, with all 6 specs accounted for.
- Pages site and embedded docs unchanged in content (check `docs/` deploy and `vocabgen serve` → Help pages).
- `make quality` green; `.kiro/` removed; archive exists outside the repo.
- §6 feedback captured.

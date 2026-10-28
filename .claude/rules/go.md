---
paths:
  - "**/*.go"
---

# Go rules

## Error handling (errcheck is enforced in CI)

- Deferred closes: `defer func() { _ = rows.Close() }()` — never bare `defer rows.Close()` / `defer resp.Body.Close()`.
- HTTP handlers: discard writer results explicitly — `_, _ = fmt.Fprintf(w, …)`, `_, _ = w.Write(…)`, `_ = renderPartial(…)`.
- Tests: `_ = os.Setenv(…)`, `_ = os.WriteFile(…)`, `_ = os.RemoveAll(…)` — or better, `t.Setenv` / `t.TempDir`.
- Compare wrapped errors with `errors.Is` / `errors.As`, never `==` (errorlint).
- HTTP and DB calls take a `context.Context` (noctx). The only allowed exceptions are those listed in `.golangci.yml`.

## Documentation (godoclint, revive)

- Each package has exactly one `// Package …` comment, in `doc.go`. A new package starts with `doc.go`.
- Godoc on every exported type, func, method and const. Unexported funcs get a comment only when the logic is non-obvious.

## Style

- Logging via `log/slog` — no `fmt.Println` in production code.
- SQL is always parameterized; never build queries by string concatenation.
- No CGo dependencies (breaks goreleaser cross-compilation).
- Refactor at 3+ duplicated call sites or a function past ~60 lines — not before.

## Tests

- Co-located `*_test.go`; table-driven by default.
- Correctness properties use `pgregory.net/rapid`; name them `TestProperty…`. Failure replays under `testdata/rapid/` are gitignored.
- No mocking frameworks — hand-written fakes that satisfy the interface (e.g. a fake `llm.Provider`).
- Web handlers are tested with `httptest`.

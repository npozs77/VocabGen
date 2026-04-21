# Contributing to VocabGen

Thanks for your interest in contributing! This is a personal project, but contributions are welcome.

## Getting Started

1. Fork the repository
2. Clone your fork and create a feature branch off `main`
3. Install Go 1.22+ and [golangci-lint](https://golangci-lint.run/welcome/install/)
4. Run `make quality` to verify everything builds and passes

## Development Workflow

```bash
# Build
make build

# Run all checks (build + vet + fmt + lint + tests with race detection + coverage)
make quality

# Run tests only
make test

# Lint (requires golangci-lint)
make lint

# Coverage report
make coverage
```

## Pull Request Process

1. Create a feature branch: `git checkout -b feature/my-change`
2. Make your changes and ensure `make quality` passes
3. Update `CHANGELOG.md` under the next version heading if the change is user-facing
4. Submit a PR against `main`

## Commit Messages

Follow the project convention:

- `feat:` — new features (always changelog-worthy)
- `fix:` — bug fixes (add `(changelog)` marker if user-facing; add `Fixes #N` to auto-close issues)
- `docs:` — documentation only
- `ci:` — CI/CD changes
- `deps:` — dependency updates

## Code Style

- Run `gofmt` / `goimports` (enforced by `make quality`)
- Run `golangci-lint run ./...` (configured in `.golangci.yml`)
- Use `log/slog` for logging, never `fmt.Println`
- Check all error returns — use `_ =` for intentionally discarded errors
- Use `defer func() { _ = x.Close() }()` for cleanup (not bare `defer x.Close()`)
- Parameterized queries for all SQL — no string concatenation

## Documentation

- Every Go package must have a `doc.go` with a package-level comment
- All exported types, functions, methods, and constants must have godoc comments
- Unexported functions with non-obvious logic should also be documented
- No duplicate `// Package` comments outside `doc.go`

## Testing

- Write table-driven tests for unit tests
- Use `pgregory.net/rapid` for property-based tests on correctness properties
- Use Go interfaces and manual mocks — no mocking frameworks
- DB tests use real SQLite via `newTestStore(t)`; handler tests use manual mock stores

## Database Migrations

If your change requires a schema change, follow the existing migration pattern in `internal/db/schema.go` — add a new `migrateVN()` function with a version check in `Migrate()`. Never modify existing migration functions.

## Reporting Issues

Use [GitHub Issues](../../issues) with the provided templates. Include steps to reproduce, expected vs actual behavior, and your OS/architecture.

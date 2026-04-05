# Contributing to VocabGen

Thanks for your interest in contributing! This is a personal project, but contributions are welcome.

## Getting Started

1. Fork the repository
2. Clone your fork and create a feature branch off `main`
3. Install Go 1.22+ and run `make quality` to verify everything builds and passes

## Development Workflow

```bash
# Build
make build

# Run all checks (build + vet + fmt-check + tests with race detection)
make quality

# Run tests only
make test

# Lint (requires golangci-lint)
make lint
```

## Pull Request Process

1. Create a feature branch: `git checkout -b feature/my-change`
2. Make your changes and ensure `make quality` passes
3. Update `CHANGELOG.md` under the next version heading if the change is user-facing
4. Submit a PR against `main`

## Commit Messages

Follow the project convention:

- `feat:` — new features (always changelog-worthy)
- `fix:` — bug fixes (add `(changelog)` marker if user-facing)
- `docs:` — documentation only
- `ci:` — CI/CD changes
- `deps:` — dependency updates

## Code Style

- Run `gofmt` (enforced by `make fmt-check`)
- Run `golangci-lint run ./...`
- Use `log/slog` for logging, never `fmt.Println`
- Check all error returns — use `_ =` for intentionally discarded errors
- Write table-driven tests; use `pgregory.net/rapid` for property-based tests

## Reporting Issues

Use [GitHub Issues](../../issues) with the provided templates. Include steps to reproduce, expected vs actual behavior, and your OS/architecture.

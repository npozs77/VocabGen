.PHONY: build dev dev-serve test lint vet fmt-check clean coverage quality e2e

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)

# Production build — outputs to ./vocabgen (used by CI and release pipeline)
build:
	cp CHANGELOG.md docs/changelog.md
	go build -ldflags "$(LDFLAGS)" -o vocabgen ./cmd/vocabgen

# Dev build — outputs to bin/vocabgen (use on feature branches)
dev:
	cp CHANGELOG.md docs/changelog.md
	go build -ldflags "$(LDFLAGS)" -o bin/vocabgen ./cmd/vocabgen

# Dev build + launch web server on port 8081 (keeps stable on 8080)
dev-serve: dev
	bin/vocabgen serve --port 8081 --db-path ~/.vocabgen/vocabgen-dev.db

test:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "--- Coverage by package ---"
	@go tool cover -func=coverage.out | grep -E '(total|^github)' | grep -v '/vendor/'
	@echo ""
	@go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$(gofmt -l .)" || { echo "Files not formatted:"; gofmt -l .; exit 1; }

clean:
	rm -f vocabgen coverage.out coverage.html
	rm -rf bin/

# Full quality gate — build, vet, format, lint, tests + coverage
quality:
	@echo "=== Build ==="
	cp CHANGELOG.md docs/changelog.md
	go build -ldflags "$(LDFLAGS)" ./cmd/vocabgen/
	@echo ""
	@echo "=== Vet ==="
	go vet ./...
	@echo ""
	@echo "=== Format ==="
	gofmt -w .
	@echo "All files formatted."
	@echo ""
	@echo "=== Lint ==="
	golangci-lint run ./...
	@echo ""
	@echo "=== Tests + Coverage ==="
	go test -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=coverage.out | tail -1

# End-to-end tests (requires E2E_PROFILE env var)
e2e:
	@E2E_PROFILE=$(E2E_PROFILE) ./scripts/e2e-test.sh

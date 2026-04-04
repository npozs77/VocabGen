.PHONY: build test lint vet fmt-check clean coverage quality e2e

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)

build:
	cp CHANGELOG.md docs/changelog.md
	go build -ldflags "$(LDFLAGS)" -o vocabgen ./cmd/vocabgen

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
	@test -z "$$(gofmt -l .)" || { echo "Files not formatted:"; gofmt -l .; exit 1; }

clean:
	rm -f vocabgen coverage.out coverage.html

quality:
	@echo "=== Build ==="
	cp CHANGELOG.md docs/changelog.md
	go build -ldflags "$(LDFLAGS)" ./cmd/vocabgen/
	@echo ""
	@echo "=== Vet ==="
	go vet ./...
	@echo ""
	@echo "=== Format Check ==="
	@test -z "$$(gofmt -l .)" || { echo "Files not formatted:"; gofmt -l .; exit 1; }
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

e2e:
	@./scripts/e2e-test.sh

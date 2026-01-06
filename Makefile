.PHONY: build run clean tidy test test-unit test-e2e lint lint-fix

# Get version info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags to inject version info
LDFLAGS := -X "gomodel/internal/version.Version=$(VERSION)" \
           -X "gomodel/internal/version.Commit=$(COMMIT)" \
           -X "gomodel/internal/version.Date=$(DATE)"

build:
	go build -ldflags '$(LDFLAGS)' -o bin/gomodel ./cmd/gomodel
# Run the application
run:
	go run ./cmd/gomodel

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy:
	go mod tidy

# Run unit tests only
test:
	go test ./internal/... ./config/... -v

# Run e2e tests (uses an in-process mock LLM server; no Docker required)
test-e2e:
	go test -v -tags=e2e ./tests/e2e/...

# Run all tests including e2e
test-all: test test-e2e

# Run linter
lint:
	golangci-lint run ./...
	golangci-lint run --build-tags=e2e ./tests/e2e/...

# Run linter with auto-fix
lint-fix:
	golangci-lint run --fix ./...

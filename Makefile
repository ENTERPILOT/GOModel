.PHONY: build run clean tidy test test-unit test-e2e lint lint-fix

# Build the application
build:
	go build -o bin/gomodel ./cmd/gomodel

# Run the application
run:
	go run ./cmd/gomodel

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy:
	go mod tidy

# Run all tests (unit only, e2e requires explicit call)
test:
	go test ./internal/... ./config/... -v

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

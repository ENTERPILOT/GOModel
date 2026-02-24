.PHONY: build run clean tidy test test-e2e test-integration test-contract test-all lint lint-fix record-api swagger

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

# Run integration tests (requires Docker for testcontainers)
test-integration:
	go test -v -tags=integration -timeout=10m ./tests/integration/...

# Run contract tests (validates API response structures against golden files)
test-contract:
	go test -v -tags=contract -timeout=5m ./tests/contract/...

# Run all tests including e2e, integration, and contract tests
test-all: test test-e2e test-integration test-contract

# Record API responses for contract tests
# Usage: OPENAI_API_KEY=sk-xxx make record-api
record-api:
	@echo "Recording OpenAI chat completion..."
	go run ./cmd/recordapi -provider=openai -endpoint=chat \
		-output=tests/contract/testdata/openai/chat_completion.json
	@echo "Recording OpenAI models..."
	go run ./cmd/recordapi -provider=openai -endpoint=models \
		-output=tests/contract/testdata/openai/models.json
	@echo "Done! Golden files saved to tests/contract/testdata/"

# Generate Swagger docs (requires swag: go install github.com/swaggo/swag/cmd/swag@latest)
swagger:
	swag init --generalInfo main.go \
		--dir cmd/gomodel,internal \
		--output cmd/gomodel/docs \
		--outputTypes go,json

# Run linter
lint:
	golangci-lint run ./...
	golangci-lint run --build-tags=e2e ./tests/e2e/...
	golangci-lint run --build-tags=integration ./tests/integration/...
	golangci-lint run --build-tags=contract ./tests/contract/...

# Run linter with auto-fix
lint-fix:
	golangci-lint run --fix ./...

.PHONY: build run clean tidy test lint lint-fix

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

# Run tests
test:
	go test ./... -v

# Run linter
lint:
	$(shell go env GOPATH)/bin/golangci-lint run ./...

# Run linter with auto-fix
lint-fix:
	$(shell go env GOPATH)/bin/golangci-lint run --fix ./...


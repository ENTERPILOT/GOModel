.PHONY: build run clean tidy test

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


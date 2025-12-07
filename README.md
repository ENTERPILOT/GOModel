# GOModel

Go! Get and use this AI Model with GOModel!

GoModel is a high-performance LLM gateway written in Go.

## Quick Start

1. Set environment variables (either via `.env` file or export):

   **Option A: Create a `.env` file:**

   ```bash
   PORT=8080
   OPENAI_API_KEY=your-api-key
   ```

   **Option B: Export environment variables:**

   ```bash
   export PORT=8080
   export OPENAI_API_KEY="your-api-key"
   ```

2. Run the server:

   ```bash
   make run
   ```

3. Test it:
   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

## Configuration

GOModel uses environment variables for configuration. You can set them either:

- In a `.env` file in the project root
- As system environment variables (takes precedence over `.env` file)

### Available Configuration Options

| Variable         | Description    | Default    |
| ---------------- | -------------- | ---------- |
| `PORT`           | Server port    | `8080`     |
| `OPENAI_API_KEY` | OpenAI API key | (required) |

## Development

### Testing

Run all tests:

```bash
make test
```

Run tests for a specific package:

```bash
go test ./config/... -v
```

### Linting

This project uses [golangci-lint](https://golangci-lint.run/) for code quality checks.

#### Installation

**macOS:**

```bash
brew install golangci-lint
```

**Linux:**

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

**Windows:**

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

For other installation methods, see the [official documentation](https://golangci-lint.run/welcome/install/).

#### Usage

Run the linter:

```bash
make lint
```

Run the linter with auto-fix:

```bash
make lint-fix
```

The linter configuration is defined in `.golangci.yml` and includes essential checks for code quality and correctness.

## Running with Docker

You can use the official `golang:1.21-alpine` image to run the project in a container:

```bash
docker run --rm -it \
  -v $(pwd):/app \
  -w /app \
  -p 8080:8080 \
  -e OPENAI_API_KEY="your-api-key" \
  golang:1.21-alpine \
  go run ./cmd/gomodel
```

## Endpoints

- `POST /v1/chat/completions` - OpenAI-compatible chat completions
- `GET /v1/models` - List available models
- `GET /health` - Health check

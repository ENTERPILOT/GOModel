# GOModel

Go! Get and use this AI Model with GOModel!

GoModel is a high-performance LLM gateway written in Go.

## Quick Start

1. Set environment variables (either via `.env` file or export):

   **Option A: Create a `.env` file:**

   ```bash
   PORT=8080
   OPENAI_API_KEY=your-openai-key
   ANTHROPIC_API_KEY=your-anthropic-key
   GEMINI_API_KEY=your-gemini-key
   ```

   **Option B: Export environment variables:**

   ```bash
   export PORT=8080
   export OPENAI_API_KEY="your-openai-key"
   export ANTHROPIC_API_KEY="your-anthropic-key"
   export GEMINI_API_KEY="your-gemini-key"
   ```

   Note: At least one API key (OpenAI, Anthropic, or Gemini) is required.

2. Run the server:

   ```bash
   make run
   ```

3. Test it:

   **OpenAI:**

   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

   **Anthropic:**

   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "claude-3-5-sonnet-20241022", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

   **Google Gemini:**

   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gemini-2.0-flash", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

   **Streaming:**

   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gemini-2.0-flash", "messages": [{"role": "user", "content": "Hello!"}], "stream": true}'
   ```

## Configuration

GOModel uses environment variables for configuration. You can set them either:

- In a `.env` file in the project root
- As system environment variables (takes precedence over `.env` file)

### Available Configuration Options

| Variable            | Description           | Default                           |
| ------------------- | --------------------- | --------------------------------- |
| `PORT`              | Server port           | `8080`                            |
| `OPENAI_API_KEY`    | OpenAI API key        | (optional, if other provider set) |
| `ANTHROPIC_API_KEY` | Anthropic API key     | (optional, if other provider set) |
| `GEMINI_API_KEY`    | Google Gemini API key | (optional, if other provider set) |

Note: At least one API key must be provided.

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
  -e OPENAI_API_KEY="your-openai-key" \
  -e ANTHROPIC_API_KEY="your-anthropic-key" \
  -e GEMINI_API_KEY="your-gemini-key" \
  golang:1.24-alpine \
  go run ./cmd/gomodel
```

Note: You can omit any API keys if you only want to use specific providers (at least one required).

## Endpoints

- `POST /v1/chat/completions` - OpenAI-compatible chat completions (supports OpenAI, Anthropic, and Gemini models)
- `GET /v1/models` - List available models from all configured providers
- `GET /health` - Health check

## Supported Providers

### OpenAI

Models starting with `gpt-` or `o1` are automatically routed to OpenAI.

Examples: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `o1-preview`, `o1-mini`

### Anthropic

Models starting with `claude-` are automatically routed to Anthropic.

Examples: `claude-3-5-sonnet-20241022`, `claude-3-5-haiku-20241022`, `claude-3-opus-20240229`

### Google Gemini

Models starting with `gemini-` are automatically routed to Google Gemini.

Examples: `gemini-2.0-flash`, `gemini-1.5-pro`, `gemini-1.5-flash`, `gemini-1.0-pro`

## Features

- **Multi-provider support**: Seamlessly use OpenAI, Anthropic, and Google Gemini models through a single API
- **Automatic routing**: Models are automatically routed to the correct provider based on their name
- **Streaming support**: All providers support streaming responses
- **OpenAI-compatible API**: Works as a drop-in replacement for OpenAI's API
- **Format conversion**: Automatically converts between provider-specific formats

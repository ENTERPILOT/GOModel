# GOModel - AI-model providers gateway written in Go

GoModel is a high-performance, easy-to-use AI gateway written in Go.

## Quick Start

1. Set environment variables (either via creating `.env` file based on `.env.template` or export):

   **Option A: Create a `.env` file:**

   ```bash
   $ cp .env.template .env
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

## Development

### Testing

```bash
make test # Run unit tests
make test-e2e # Run e2e tests
make test-all # Run all tests (unit tests, e2e tests):
```

### Linting

This project uses [golangci-lint](https://golangci-lint.run/) for code quality checks.

#### Linter installation

See the [official golangci-lint documentation](https://golangci-lint.run/welcome/install/).

#### Usage

```bash
make lint # check the code quality
make lint-fix # try to fix the code automatically
```

## Running with Docker

You can use the official `golang:1.24-alpine` image to run the project in a container:

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

## Supported Providers

| Provider      | Status |
| ------------- | ------ |
| OpenAI        | ✅     |
| Anthropic     | ✅     |
| Google Gemini | ✅     |
| OpenRouter    | 🔜     |
| Groq          | 🔜     |
| xAI           | 🔜     |

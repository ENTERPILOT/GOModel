# GOModel - AI-model providers gateway written in Go

GoModel is a high-performance, easy-to-use AI gateway written in Go.

## Quick Start

### Running Manually

1. Set environment variables (either via creating `.env` file based on `.env.template` or export):

   **Option A: Create a `.env` file based on `.env.template`:**

   ```bash
   $ cp .env.template .env
   ```

   **Option B: Export environment variables:**

   ```bash
   export OPENAI_API_KEY="your-openai-key"
   export ANTHROPIC_API_KEY="your-anthropic-key"
   export GEMINI_API_KEY="your-gemini-key"
   ```

   Note: At least one API key (OpenAI, Anthropic, Gemini, etc.) is required.

2. Run the server:

   ```bash
   make run
   ```

3. (optionally) Test it:

   **OpenAI:**

   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gpt-5-nano", "messages": [{"role": "user", "content": "Hello!"}]}'
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

### Running with Docker

You can use the official `golang:1.24-alpine` image to run the project in a container:

```bash
make build
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

### Running with Docker Compose

```bash
$ cp .env.template .env
# fill envs ...
$ make build
$ docker compose up -d
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

### Pre-commit

You can install predefined pre-commit checks with [pre-commit CLI tool](https://pre-commit.com/). To do so, use the following commands or [follow the official pre-commit documentation](https://pre-commit.com/#install):

```bash
pip install pre-commit
pre-commit install
```

# Roadmap

## Supported Providers

| Provider      | Basic support | Pass-through      | Voice models      | Image gen         | Image gen         | Full /responses API | Embedding         | Caching           |
| ------------- | ------------- | ----------------- | ----------------- | ----------------- | ----------------- | ------------------- | ----------------- | ----------------- |
| OpenAI        | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |
| Anthropic     | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |
| Google Gemini | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |
| OpenRouter    | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |
| Groq          | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |
| xAI           | ✅            | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon... | 🚧 Coming soon...   | 🚧 Coming soon... | 🚧 Coming soon... |

## Features

| Feature                    | Basic support     | Full support      |
| -------------------------- | ----------------- | ----------------- |
| Billing Management         | 🚧 Coming soon... | 🚧 Coming soon... |
| Full-observibility         | 🚧 Coming soon... | 🚧 Coming soon... |
| Budget management          | 🚧 Coming soon... | 🚧 Coming soon... |
| Many keys support          | 🚧 Coming soon... | 🚧 Coming soon... |
| Administrative endpoints   | 🚧 Coming soon... | 🚧 Coming soon... |
| Guardrails                 | 🚧 Coming soon... | 🚧 Coming soon... |
| SSO                        | 🚧 Coming soon... | 🚧 Coming soon... |
| System Prompt (GuardRails) | 🚧 Coming soon... | 🚧 Coming soon... |

## Integrations

| Integration   | Basic integration | Full support      |
| ------------- | ----------------- | ----------------- |
| Prometheus    | ✅                | 🚧 Coming soon... |
| DataDog       | 🚧 Coming soon... | 🚧 Coming soon... |
| OpenTelemetry | 🚧 Coming soon... | 🚧 Coming soon... |

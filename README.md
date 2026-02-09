# GOModel

A high-performance AI gateway written in Go. Unified OpenAI-compatible API for multiple LLM providers.

## Quick Start

**Step 1:** Start GOModel

```bash
docker run --rm -p 8080:8080 \
  -e GEMINI_API_KEY="your-gemini-key" \
  enterpilot/gomodel
```

Pass only the API keys you need (at least one required):

```bash
docker run --rm -p 8080:8080 \
  -e OPENAI_API_KEY="your-openai-key" \
  -e ANTHROPIC_API_KEY="your-anthropic-key" \
  -e GEMINI_API_KEY="your-gemini-key" \
  -e GROQ_API_KEY="your-groq-key" \
  enterpilot/gomodel
```

**Step 2:** Make your first API call

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**That's it!** GOModel automatically detects which providers are available based on the API keys you supply.

### Supported Providers

| Provider | Environment Variable | Example Model |
|----------|---------------------|---------------|
| OpenAI | `OPENAI_API_KEY` | `gpt-4o-mini` |
| Anthropic | `ANTHROPIC_API_KEY` | `claude-3-5-sonnet-20241022` |
| Google Gemini | `GEMINI_API_KEY` | `gemini-2.5-flash` |
| Groq | `GROQ_API_KEY` | `llama-3.3-70b-versatile` |
| xAI (Grok) | `XAI_API_KEY` | `grok-2` |
| Ollama | `OLLAMA_BASE_URL` | `llama3.2` |

---

## Alternative Setup Methods

### Running from Source

**Prerequisites:** Go 1.22+

1. Create a `.env` file:

   ```bash
   cp .env.template .env
   ```

2. Add your API keys to `.env` (at least one required).

3. Start the server:

   ```bash
   make run
   ```

### Docker Compose (Full Stack)

Includes GOModel + Redis + PostgreSQL + MongoDB + Adminer + Prometheus:

```bash
cp .env.template .env
# Add your API keys to .env
docker compose up -d
```

| Service | URL |
|---------|-----|
| GOModel API | http://localhost:8080 |
| Adminer (DB UI) | http://localhost:8081 |
| Prometheus | http://localhost:9090 |

### Building the Docker Image Locally

```bash
docker build -t gomodel .
docker run --rm -p 8080:8080 -e GEMINI_API_KEY="your-key" gomodel
```

---

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (streaming supported) |
| `/v1/responses` | POST | OpenAI Responses API |
| `/v1/models` | GET | List available models |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics (when enabled) |

---

## Configuration

GOModel is configured through environment variables. See [`.env.template`](.env.template) for all options.

Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `GOMODEL_MASTER_KEY` | (none) | API key for authentication |
| `CACHE_TYPE` | `local` | Cache backend (`local` or `redis`) |
| `STORAGE_TYPE` | `sqlite` | Storage backend (`sqlite`, `postgresql`, `mongodb`) |
| `METRICS_ENABLED` | `false` | Enable Prometheus metrics |
| `LOGGING_ENABLED` | `false` | Enable audit logging |

---

## Development

### Testing

```bash
make test          # Unit tests
make test-e2e      # End-to-end tests
make test-all      # All tests
```

### Linting

Requires [golangci-lint](https://golangci-lint.run/welcome/install/).

```bash
make lint          # Check code quality
make lint-fix      # Auto-fix issues
```

### Pre-commit

```bash
pip install pre-commit
pre-commit install
```

---

## Roadmap

### Supported Providers

| Provider | Basic support | Pass-through | Voice models | Image gen | Video gen | Full /responses API | Embedding | Caching |
|----------|---------------|--------------|--------------|-----------|-----------|---------------------|-----------|---------|
| OpenAI | ✅ | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |
| Anthropic | ✅ | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |
| Google Gemini | ✅ | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |
| OpenRouter | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |
| Groq | ✅ | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |
| xAI | ✅ | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 | 🚧 |

### Features

| Feature                    | Basic support      | Full support       |
| -------------------------- | ------------------ | ------------------ |
| Billing Management         | 🚧 Coming soon... | 🚧 Coming soon... |
| Full-observability         | 🚧 Coming soon... | 🚧 Coming soon... |
| Budget management          | 🚧 Coming soon... | 🚧 Coming soon... |
| Many keys support          | 🚧 Coming soon... | 🚧 Coming soon... |
| Administrative endpoints   | 🚧 Coming soon... | 🚧 Coming soon... |
| Guardrails                 | 🚧 Coming soon... | 🚧 Coming soon... |
| SSO                        | 🚧 Coming soon... | 🚧 Coming soon... |
| System Prompt (GuardRails) | 🚧 Coming soon... | 🚧 Coming soon... |

### Integrations

| Integration   | Basic integration  | Full support       |
| ------------- | ------------------ | ------------------ |
| Prometheus    | ✅                 | 🚧 Coming soon... |
| DataDog       | 🚧 Coming soon... | 🚧 Coming soon... |
| OpenTelemetry | 🚧 Coming soon... | 🚧 Coming soon... |

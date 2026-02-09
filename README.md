# GOModel

A high-performance AI gateway written in Go. Unified OpenAI-compatible API for multiple LLM providers.

## Quick Start

**Step 1:** Start GOModel

```bash
docker run --rm -p 8080:8080 \
  -e GEMINI_API_KEY="your-gemini-key" \
  enterpilot/gomodel
```

Pass only the provider credentials or base URL you need (at least one required):

```bash
docker run --rm -p 8080:8080 \
  -e OPENAI_API_KEY="your-openai-key" \
  -e ANTHROPIC_API_KEY="your-anthropic-key" \
  -e GEMINI_API_KEY="your-gemini-key" \
  -e GROQ_API_KEY="your-groq-key" \
  -e XAI_API_KEY="your-xai-key" \
  -e OLLAMA_BASE_URL="http://host.docker.internal:11434/v1" \
  enterpilot/gomodel
```

⚠️ Avoid passing secrets via `-e` on the command line—they can leak via shell history and process lists. For production, use `docker run --env-file .env` to load API keys from a file instead.

**Step 2:** Make your first API call

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**That's it!** GOModel automatically detects which providers are available based on the credentials you supply.

### Supported Providers

<table>
  <tr>
    <th colspan="3">Provider</th>
    <th colspan="8">Features</th>
  </tr>
  <tr>
    <th style="white-space: nowrap">Name</th>
    <th>Credential</th>
    <th style="white-space: nowrap">Example Model</th>
    <th>Chat</th>
    <th>Passthru</th>
    <th>Voice</th>
    <th>Image</th>
    <th>Video</th>
    <th>/responses</th>
    <th>Embed</th>
    <th>Cache</th>
  </tr>
  <tr>
    <td>OpenAI</td>
    <td><code>OPENAI_API_KEY</code></td>
    <td style="white-space: nowrap"><code>gpt-4o-mini</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
  <tr>
    <td>Anthropic</td>
    <td><code>ANTHROPIC_API_KEY</code></td>
    <td style="white-space: nowrap"><code>claude-3-5-sonnet-20241022</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
  <tr>
    <td style="white-space: nowrap">Google Gemini</td>
    <td><code>GEMINI_API_KEY</code></td>
    <td style="white-space: nowrap"><code>gemini-2.5-flash</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
  <tr>
    <td>Groq</td>
    <td><code>GROQ_API_KEY</code></td>
    <td style="white-space: nowrap"><code>llama-3.3-70b-versatile</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
  <tr>
    <td style="white-space: nowrap">xAI (Grok)</td>
    <td><code>XAI_API_KEY</code></td>
    <td style="white-space: nowrap"><code>grok-2</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
  <tr>
    <td>Ollama</td>
    <td><code>OLLAMA_BASE_URL</code></td>
    <td style="white-space: nowrap"><code>llama3.2</code></td>
    <td>✅</td><td>🚧</td><td>🚧</td><td>—</td><td>—</td><td>🚧</td><td>🚧</td><td>🚧</td>
  </tr>
</table>

✅ Supported  🚧 Coming soon  — Not applicable

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
docker run --rm -p 8080:8080 --env-file .env gomodel
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

**Quick Start — Authentication:** By default `GOMODEL_MASTER_KEY` is unset. Without this key, API endpoints are unprotected and anyone can call them. This is insecure for production. **Strongly recommend** setting a strong secret before exposing the service. Add `GOMODEL_MASTER_KEY` to your `.env` or environment for production deployments.

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

# Roadmap

## Features

| Feature                    | Basic support     | Full support      |
| -------------------------- | ----------------- | ----------------- |
| Billing Management         | 🚧 Coming soon... | 🚧 Coming soon... |
| Full-observability         | 🚧 Coming soon... | 🚧 Coming soon... |
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

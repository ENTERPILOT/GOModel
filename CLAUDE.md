# CLAUDE.md

Guidance for AI models (like Claude) working with this codebase. Some information may be slightly outdated—verify current structure as needed.

## Project Overview

**GOModel** is a high-performance AI gateway in Go that routes requests to multiple LLM providers (OpenAI, Anthropic, Gemini, Groq, xAI). Drop-in LiteLLM replacement.

- **Module:** `gomodel` | **Go:** 1.24.0 | **Repo:** https://github.com/ENTERPILOT/GOModel
- **Stage:** Development—backward compatibility is not a concern

## Commands

```bash
make run          # Run server (requires .env with API key)
make build        # Build to bin/gomodel
make test         # Unit tests only
make test-e2e     # E2E tests (in-process mock, no Docker)
make test-all     # All tests
make lint         # Run golangci-lint
make lint-fix     # Auto-fix lint issues
```

**Single test:** `go test ./internal/providers -v -run TestName`
**E2E single test:** `go test -v -tags=e2e ./tests/e2e/... -run TestName`

## Architecture

**Request flow:** `Client → Echo Handler → Router → Provider Adapter → Upstream API`

**Core components:**
- `internal/providers/registry.go` — Model-to-provider mapping, local/Redis cache, hourly background refresh
- `internal/providers/router.go` — Routes by model name, returns `ErrRegistryNotInitialized` if used before init
- `internal/providers/factory.go` — Provider instantiation via explicit `factory.Register()` calls
- `internal/core/interfaces.go` — `Provider` interface (ChatCompletion, StreamChatCompletion, ListModels, Responses, StreamResponses)

**Startup:** Load from cache → start server → refresh models in background

**Config** (via `.env` created from `.env.template`, and `config/config.yaml`):
- `PORT` (default 8080), `CACHE_TYPE` (local/redis), `REDIS_URL`, `REDIS_KEY`
- `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY` — at least one required

## Adding a Provider

1. Create `internal/providers/{name}/` implementing `core.Provider`
2. Export a `Registration` variable with type and constructor
3. Register in `cmd/gomodel/main.go` via `factory.Register({name}.Registration)`
4. Add API key to `.env.template`

## Project Structure

```
cmd/gomodel/           # Entrypoint
config/                # Viper config loading
internal/
  core/                # Interfaces, types
  providers/           # Provider implementations, router, registry, factory
  cache/               # Local/Redis cache backends
  server/              # Echo HTTP server, handlers
  observability/       # Prometheus metrics
tests/e2e/             # E2E tests (requires -tags=e2e)
```

## Testing

- **Unit tests:** Alongside implementation files (`*_test.go`)
- **E2E tests:** In-process mock server, no Docker required
- **Manual storage testing:** Docker Compose is optional, for manual validation only

```bash
# Connect local GOModel to Dockerized DB for manual testing
STORAGE_TYPE=postgresql POSTGRES_URL=postgres://gomodel:gomodel@localhost:5432/gomodel go run ./cmd/gomodel
STORAGE_TYPE=mongodb MONGODB_URL=mongodb://localhost:27017/gomodel go run ./cmd/gomodel
```

Note that Docker Compose is optional and intended solely for manual storage-backend validation; automated unit and E2E tests must run in-process without Docker (see `make test` and `make test-e2e`).

## Key Details

1. Providers are registered explicitly via `factory.Register()` in main.go
2. Router requires initialized registry—check `ModelCount() > 0`
3. Streaming returns `io.ReadCloser`—caller must close
4. First registered provider wins for shared models
5. Models auto-refresh hourly by default (configurable via `RefreshInterval`)

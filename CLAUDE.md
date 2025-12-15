# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GOModel is a high-performance AI gateway written in Go that routes requests to multiple LLM providers (OpenAI, Anthropic, Google Gemini). It's designed as a drop-in replacement for LiteLLM with superior concurrency and strict type safety.

**Module name:** `gomodel`
**Go version:** 1.24.0

## Common Development Commands

### Running the Server

```bash
make run                 # Run the server (requires .env with at least one API key)
go run ./cmd/gomodel     # Direct run without make
```

### Building

```bash
make build              # Builds binary to bin/gomodel
make clean              # Removes build artifacts
```

### Testing

```bash
make test               # Run unit tests only (internal/... and config/...)
make test-e2e           # Run e2e tests (uses in-process mock server, no Docker)
make test-all           # Run all tests (unit + e2e)
```

**Running a single test:**

```bash
go test ./internal/providers -v -run TestSpecificFunction
go test -v -tags=e2e ./tests/e2e/... -run TestSpecificE2E
```

### Linting

```bash
make lint               # Run golangci-lint on all code
make lint-fix           # Auto-fix linting issues
```

**Note:** E2E tests require the `e2e` build tag. The linter runs twice (once for main code, once for e2e tests).

### Dependencies

```bash
make tidy               # Run go mod tidy
```

## Architecture Overview

### Request Flow

The system operates as a pipeline processor:

```
Client -> HTTP Handler (Echo) -> Router -> Provider Adapter (OpenAI/Anthropic/Gemini) -> Upstream API
```

### Core Components

**Provider Registry** (`internal/providers/registry.go`)

- Central model-to-provider mapping system
- Fetches available models from each provider's `/models` endpoint at startup
- Supports both local file cache and Redis cache for instant startup
- Background refresh every 5 minutes to keep model lists current
- Thread-safe with RWMutex for concurrent access

**Router** (`internal/providers/router.go`)

- Routes requests to correct provider based on model name
- Uses the ModelRegistry to determine which provider handles each model
- Returns `ErrRegistryNotInitialized` if used before registry has models

**Provider Interface** (`internal/core/interfaces.go`)

```go
type Provider interface {
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    StreamChatCompletion(ctx context.Context, req *ChatRequest) (io.ReadCloser, error)
    ListModels(ctx context.Context) (*ModelsResponse, error)
    Responses(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error)
    StreamResponses(ctx context.Context, req *ResponsesRequest) (io.ReadCloser, error)
}
```

**Provider Factory** (`internal/providers/factory.go`)

- Dynamically instantiates providers based on configuration
- Uses init() registration pattern (see below)

### Initialization Flow

1. **main.go** loads config and creates cache backend (local or Redis)
2. Provider packages are imported with blank imports (`_ "gomodel/internal/providers/openai"`)
3. Each provider's `init()` registers itself with the factory
4. main.go creates providers via factory and registers them with the ModelRegistry
5. **Non-blocking startup:** Registry loads from cache first, then refreshes in background
6. Server starts immediately with cached models while fresh data is fetched
7. Background goroutine refreshes models every 5 minutes and updates cache

### Configuration

**Environment variables** (via .env or export):

- `PORT`: Server port (default 8080)
- `CACHE_TYPE`: "local" or "redis"
- `REDIS_URL`, `REDIS_KEY`: Redis configuration if using redis cache
- `GOMODEL_CACHE_DIR`: Override cache directory (default: ./.cache)
- `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`: Provider credentials

**Config loading** (`config/config.go`):

- Uses Viper to load from environment variables and .env file
- Automatically creates provider configs from environment variables
- At least one provider API key is required

### Adding a New Provider

1. Create package in `internal/providers/{name}/`
2. Implement the `core.Provider` interface
3. Add `init()` function to register with factory:
   ```go
   func init() {
       providers.RegisterFactory("provider-name", NewProvider)
   }
   ```
4. Import provider in `cmd/gomodel/main.go` with blank import
5. Add API key environment variable to `.env.template`

### Key Design Principles

**Pragmatic Modularity:** Avoid over-abstraction. Use flat, composable packages instead of deep hierarchies.

**Concurrency:** Heavy use of goroutines and channels for high-throughput scenarios (10k+ concurrent connections).

**Strict Typing:** All request/response payloads use strongly-typed structs to catch errors at compile time.

**Non-blocking I/O:** The registry loads from cache synchronously, then refreshes asynchronously to enable instant startup.

## Project Structure

```
cmd/gomodel/          # Application entrypoint
config/               # Configuration structs (Viper + env loading)
internal/
  core/               # Core interfaces and types (Provider, ChatRequest, etc.)
  providers/          # Provider implementations and routing
    openai/           # OpenAI provider
    anthropic/        # Anthropic provider
    gemini/           # Google Gemini provider
    factory.go        # Provider factory with registration
    registry.go       # Model registry with caching
    router.go         # Request router
  cache/              # Cache backends (local file, Redis)
  pkg/                # Internal shared packages
    httpclient/       # HTTP client utilities
    llmclient/        # LLM-specific client logic
  server/             # HTTP server (Echo framework)
    http.go           # Server setup
    handlers.go       # Request handlers
tests/e2e/            # End-to-end tests (requires -tags=e2e)
```

## Testing Notes

- **Unit tests:** Located alongside implementation files (`*_test.go`)
- **E2E tests:** Require `-tags=e2e` build tag and use in-process mock LLM server
- No Docker or external dependencies needed for testing
- Mock provider available in `tests/e2e/mock_provider.go`

## HTTP Framework

Uses **Echo (v4)** for the HTTP layer:

- Robust request context
- Built-in middleware support
- Better suited for gateway architecture than Chi

## Cache System

**Purpose:** Store model-to-provider mappings to enable instant startup

**Backends:**

- **Local** (default): JSON file in `.cache/models.json`
- **Redis**: Shared cache for multi-instance deployments

**Cache structure:**

```go
type ModelCache struct {
    Version   int
    UpdatedAt time.Time
    Models    map[string]CachedModel  // modelID -> cached model info
}
```

## Important Implementation Details

1. **Provider registration uses init()**: Providers self-register when imported
2. **Router requires initialized registry**: Always check `ModelCount() > 0` or use after `InitializeAsync()`
3. **Streaming responses return io.ReadCloser**: Caller must close the stream
4. **First provider wins**: If multiple providers support same model, first registered wins
5. **Background refresh**: Models auto-refresh every 5 minutes without downtime

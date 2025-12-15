# Architecture Draft: GOModel (v1)

**Goal:** A high-performance, drop-in replacement for LiteLLM with superior concurrency and strict type safety.
**Philosophy:** Pragmatic Modularity. Avoid "Java-style" over-abstraction (no deep infrastructure/persistence/impl trees). Focus on flat, composable packages.

---

## 1. High-Level Design

GOModel functions as a pipeline processor. Every request travels through a configurable chain of middleware before reaching the Provider adapter.

**Flow:**
`Client -> HTTP Handler -> Middleware Chain (Auth, RateLimit) -> Router -> Provider Adapter (OpenAI/Anthropic) -> Upstream`

---

## 2. Directory Structure

We use a standard Go layout but flattened for developer velocity.

```

gomodel/
├── cmd/
│   └── gomodel/
│       └── main.go           # Entrypoint: Loads config, wires dependencies, starts server.
│
├── config/                   # YAML/Env configuration structs (Viper).
│   ├── config.go
│   └── config.yaml           # Default configuration.
│
├── internal/
│   ├── core/                 # The "Contracts". No external deps.
│   │   ├── types.go          # Common structs (ChatRequest, ChatResponse, Usage).
│   │   └── interfaces.go     # Core interfaces (Provider, Middleware, Logger).
│   │
│   ├── providers/            # LLM Logic (The Adapters).
│   │   ├── openai/           # OpenAI specific implementation. ✅ IMPLEMENTED
│   │   │   └── openai.go     # OpenAI provider with chat completion and streaming.
│   │   ├── anthropic/        # Anthropic specific implementation. ✅ IMPLEMENTED
│   │   │   └── anthropic.go  # Anthropic provider with format conversion.
│   │   ├── factory.go        # Logic to instantiate providers by string name.
│   │   ├── base.go           # Shared logic (HTTP clients, retries).
│   │   └── router.go         # Routes requests to appropriate provider.
│   │
│   ├── router/               # "Brain" logic.
│   │   └── router.go         # Handles model aliasing, fallbacks, and load balancing.
│   │
│   ├── middleware/           # Interceptors (The Plugin System).
│   │   ├── chain.go          # Logic to chain middleware functions.
│   │   ├── logging.go        # Structured logging (Slog).
│   │   ├── ratelimit.go      # Redis/Memory token bucket.
│   │   └── pii.go            # PII scrubbing logic.
│   │
│   └── server/               # HTTP Transport Layer.
│       ├── http.go           # Setup Echo/Chi router.
│       └── handlers.go       # Map /v1/chat/completions to Router.
│
├── pkg/                      # Code usable by OTHER Go projects (SDK).
│   └── api/                  # Public request/response definitions.
│
├── deploy/                   # Dockerfiles, Compose, Helm.
├── go.mod
└── Makefile

```

---

## 3. Technology Stack

### Core Frameworks

**Web Framework:** `labstack/echo (v4)` or `go-chi/chi`
**Reason:** Chi is lighter/standard, but Echo provides a more robust context and built-in middleware which speeds up development of a Gateway. _Assume Echo for performance + features._

**Configuration:** `spf13/viper`
**Reason:** Industry standard for loading config from Env, YAML, and Flags simultaneously.

**Logging:** `log/slog` (Go Standard Lib)
**Reason:** Zero dependency, structured JSON logging, fast.

### High-Performance Libraries

**JSON:** `goccy/go-json` or `sonic`
**Reason:** 2–3× faster than standard `encoding/json`. Critical for high-throughput heavy payloads.

**HTTP Client:** `valyala/fasthttp` (if extreme perf needed) or standard `net/http` with connection pooling.
**Decision:** Start with `net/http` tuned (KeepAlives, MaxIdleConns) for stability, swap later if needed.

---

## 4. Key Architectural Patterns

### A. The Provider Interface

This allows adding new models (Gemini, Mistral) without touching the core server code.

```go
// internal/core/interfaces.go
type Provider interface {
    // Determine if this provider can handle the model name
    Supports(model string) bool

    // The main execution method
    ChatCompletion(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)

    // For streaming responses
    StreamChatCompletion(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamChunk, error)
}
```

---

### B. The Middleware Chain

Unlike typical HTTP middleware, LLM-aware middleware runs **after request parsing** but **before the provider call** so it can access the `ChatRequest` struct.

```go
// internal/middleware/types.go
type LLMMiddleware func(next core.Provider) core.Provider
```

This allows wrapping providers like:

```
Auth(RateLimit(Logging(OpenAIProvider)))
```

---

### C. Unified Configuration

A single `config.yaml` maps **Virtual Models** to **Real Providers**.

```yaml
models:
  - name: "gpt-corporate" # What the user asks for
    routing: "load-balance" # Strategy
    targets: # Where it actually goes
      - provider: "openai"
        model: "gpt-4-turbo"
        weight: 80
      - provider: "azure"
        model: "gpt-4"
        weight: 20
```

---

## 5. Easy migration from LiteLLM to GOModel.

The config.yml config file should be LiteLLM config compatible.

## 6. Why This Beats LiteLLM?

**Single Binary:**
Go compiles to a static binary. No Python environment, pip install, or dependency hell.

**Concurrency:**
LiteLLM (Python) uses asyncio. GOModel uses Goroutines — enabling 10k+ concurrent connections with far lower RAM.

**Strict Typing:**
Request/Response payloads are strictly validated structs, reducing runtime errors significantly.

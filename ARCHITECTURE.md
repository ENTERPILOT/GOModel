# Architecture: GOModel

**Goal:** A high-performance, modular AI gateway inspired by LiteLLM, with superior concurrency, strict type safety, and enterprise features.

**Philosophy:** Pragmatic Modularity. Every component is optional except the core. Speed and quality over features.

---

## 1. High-Level Design

GOModel functions as a pipeline processor with configurable middleware chains:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              INGRESS                                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │  /v1/* Gateway  │  │ /[provider]/*   │  │  /admin/*                   │  │
│  │  Unified API    │  │ Pass-through    │  │  Management API             │  │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────────────────┘  │
└───────────┼─────────────────────┼───────────────────────────────────────────┘
            │                     │
            ▼                     ▼
┌───────────────────────┐  ┌─────────────────┐
│   MIDDLEWARE CHAIN    │  │  PASS-THROUGH   │
│  Auth → RateLimit →   │  │  (Auth, Audit,  │
│  Budget → Guardrails  │  │   Metrics only) │
│  → Cache              │  └────────┬────────┘
└───────────┬───────────┘           │
            │                       │
            ▼                       │
┌───────────────────────────────────┼─────────────────────────────────────────┐
│                    ROUTING LAYER  │                                          │
│  Model Registry ←→ Failover Manager ←→ Load Balancer                        │
└───────────────────────────────────┼─────────────────────────────────────────┘
            │                       │
            ▼                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         PROVIDER LAYER                                       │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  Keyring: Multi-key rotation, per-key limits, circuit breakers       │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│  OpenAI │ Anthropic │ Gemini │ Groq │ xAI │ OpenRouter │ ...               │
└─────────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  POST-PROCESSING: Guardrails(out) → Usage Tracking → Cache Store → Audit   │
└─────────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         STORAGE & LOGGING                                    │
│  Credentials (env/file/db/vault) │ Audit (file/mongo/elastic/datadog)       │
│  Cache (memory/redis)            │ Metrics (prometheus/otel/datadog)        │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Directory Structure

```
gomodel/
├── cmd/
│   └── gomodel/
│       └── main.go                 # Entrypoint: wire dependencies, start server
│
├── config/
│   ├── config.go                   # Viper-based config loading
│   ├── config.yaml                 # Main configuration (optional)
│   ├── failover.yaml               # Failover chains
│   └── guardrails.yaml             # Guardrail configurations
│
├── internal/
│   ├── core/                       # Contracts (zero external deps)
│   │   ├── interfaces.go           # Provider, Middleware, Storage interfaces
│   │   ├── types.go                # ChatRequest, ChatResponse, Usage
│   │   └── errors.go               # Unified error types
│   │
│   ├── middleware/                 # Request/Response interceptors
│   │   ├── chain.go                # Middleware composition
│   │   ├── auth.go                 # API key + JWT + SSO
│   │   ├── ratelimit.go            # Token bucket (memory/Redis)
│   │   ├── budget.go               # Budget enforcement
│   │   └── cache.go                # Response caching
│   │
│   ├── guardrails/                 # Pluggable guardrail system
│   │   ├── interface.go            # Guardrail interface
│   │   ├── chain.go                # Guardrail chain executor
│   │   ├── pii.go                  # PII detection/redaction
│   │   ├── content.go              # Content moderation
│   │   ├── tokens.go               # Token limits
│   │   ├── regex.go                # Custom pattern filtering
│   │   └── prompt.go               # System prompt injection
│   │
│   ├── providers/                  # LLM Adapters
│   │   ├── openai/
│   │   ├── anthropic/
│   │   ├── gemini/
│   │   ├── groq/
│   │   ├── xai/
│   │   ├── openrouter/
│   │   ├── registry.go             # Model → Provider mapping
│   │   ├── router.go               # Request routing
│   │   ├── keyring.go              # Multi-key management
│   │   ├── factory.go              # Provider instantiation
│   │   └── responses_converter.go  # Shared stream converter
│   │
│   ├── routing/                    # Advanced routing
│   │   ├── failover.go             # Failover chain execution
│   │   ├── loadbalancer.go         # Weighted routing
│   │   └── aliases.go              # Model aliasing
│   │
│   ├── credentials/                # Credential sources
│   │   ├── interface.go            # CredentialStore interface
│   │   ├── env.go                  # Environment variables (default)
│   │   ├── file.go                 # YAML file
│   │   ├── postgres.go             # PostgreSQL (optional)
│   │   └── vault.go                # HashiCorp Vault (optional)
│   │
│   ├── audit/                      # Request/Response logging
│   │   ├── interface.go            # AuditLogger interface
│   │   ├── noop.go                 # Disabled (default)
│   │   ├── file.go                 # Local JSON files
│   │   ├── mongodb.go              # MongoDB
│   │   ├── elasticsearch.go        # Elasticsearch
│   │   └── datadog.go              # DataDog Logs API
│   │
│   ├── billing/                    # Usage tracking
│   │   ├── tracker.go              # Usage accumulation
│   │   ├── budget.go               # Budget limits
│   │   └── export.go               # Usage export
│   │
│   ├── admin/                      # Admin API
│   │   ├── handlers.go             # CRUD endpoints
│   │   ├── users.go                # User/Team management
│   │   └── keys.go                 # API key management
│   │
│   ├── cache/                      # Caching backends
│   │   ├── interface.go
│   │   ├── local.go                # File-based
│   │   └── redis.go                # Redis
│   │
│   ├── observability/              # Telemetry
│   │   ├── metrics.go              # Prometheus
│   │   ├── tracing.go              # OpenTelemetry
│   │   ├── datadog.go              # DataDog APM
│   │   └── hooks.go                # Provider hooks
│   │
│   ├── llmclient/                  # Base LLM HTTP client
│   │   └── client.go               # Retries, circuit breaker
│   │
│   ├── httpclient/                 # HTTP utilities
│   │   └── client.go               # Connection pooling
│   │
│   └── server/                     # HTTP layer
│       ├── http.go                 # Echo setup
│       ├── handlers.go             # /v1/* handlers
│       ├── passthrough.go          # /[provider]/* handlers
│       └── admin_handlers.go       # /admin/* handlers
│
└── tests/
    ├── e2e/                        # End-to-end with mocks
    ├── integration/                # Against real providers
    └── load/                       # Performance benchmarks
```

---

## 3. Core Interfaces

### Provider Interface

```go
// internal/core/interfaces.go
type Provider interface {
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    StreamChatCompletion(ctx context.Context, req *ChatRequest) (io.ReadCloser, error)
    ListModels(ctx context.Context) (*ModelsResponse, error)
    Responses(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error)
    StreamResponses(ctx context.Context, req *ResponsesRequest) (io.ReadCloser, error)
}
```

### Middleware Interface (LLM-Aware)

```go
// internal/core/interfaces.go
type Middleware interface {
    Process(ctx context.Context, req *ChatRequest, next Handler) (*ChatResponse, error)
}

type Handler func(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

// Chain composes middleware
func Chain(middlewares ...Middleware) Middleware
```

### Guardrail Interface

```go
// internal/guardrails/interface.go
type Guardrail interface {
    Name() string
    Priority() int

    // Pre-provider
    ProcessInput(ctx context.Context, req *ChatRequest) (*ChatRequest, error)

    // Post-provider
    ProcessOutput(ctx context.Context, req *ChatRequest, resp *ChatResponse) (*ChatResponse, error)
}
```

### Storage Interfaces

```go
// internal/credentials/interface.go
type CredentialStore interface {
    GetProviderKeys(ctx context.Context, provider string) ([]APIKey, error)
}

// internal/audit/interface.go
type AuditLogger interface {
    LogRequest(ctx context.Context, entry *RequestEntry) error
    LogResponse(ctx context.Context, entry *ResponseEntry) error
    LogMiddlewareStep(ctx context.Context, entry *MiddlewareEntry) error
}
```

---

## 4. Configuration

### Main Configuration

```yaml
# config/config.yaml
server:
  port: 8080

features:
  middleware: true
  guardrails: true
  audit_logging: false
  admin_api: false
  failover: true

credentials:
  source: env  # env, file, postgres, vault

cache:
  backend: local  # local, redis

audit:
  backend: noop  # noop, file, mongodb, elasticsearch, datadog
```

### Failover Configuration

```yaml
# config/failover.yaml
failover:
  openai/gpt-5:
    - openai/gpt-5-mini
    - openrouter/gpt-5
    - anthropic/claude-3-opus

  anthropic/claude-3-opus:
    - anthropic/claude-3-sonnet
    - openai/gpt-4-turbo

  # Provider-level fallback
  openai/*:
    - openrouter/*
    - azure/*

triggers:
  status_codes: [429, 500, 502, 503, 504]
  timeout_ms: 30000
```

### Guardrails Configuration

```yaml
# config/guardrails.yaml
guardrails:
  - name: pii-redactor
    enabled: true
    priority: 10
    config:
      detect: [email, phone, ssn, credit_card]
      action: redact

  - name: token-limiter
    enabled: true
    priority: 20
    config:
      max_input_tokens: 8000
      max_output_tokens: 4000

  - name: system-prompt
    enabled: true
    priority: 5
    config:
      prepend: "You are a helpful assistant."

  - name: content-filter
    enabled: true
    priority: 30
    config:
      block_categories: [hate, violence, self_harm]
```

---

## 5. API Routes

### Gateway API (Unified)

```
POST   /v1/chat/completions     # Model-routed chat
GET    /v1/models               # List all models
POST   /v1/responses            # OpenAI Responses API
POST   /v1/embeddings           # Embeddings (future)
POST   /v1/images/generations   # Image gen (future)
```

### Pass-through API

```
/*     /openai/*                # → api.openai.com
/*     /anthropic/*             # → api.anthropic.com
/*     /gemini/*                # → generativelanguage.googleapis.com
/*     /groq/*                  # → api.groq.com
/*     /xai/*                   # → api.x.ai
/*     /openrouter/*            # → openrouter.ai
```

Reserved paths (cannot be provider names):
- `/v1`, `/v2` - Gateway API versions
- `/admin` - Admin API
- `/health`, `/healthz`, `/ready`, `/live` - Health checks
- `/metrics` - Prometheus metrics

### Admin API

```
GET    /admin/users             # List users/teams
POST   /admin/users             # Create user
GET    /admin/keys              # List API keys
POST   /admin/keys              # Create API key
GET    /admin/budgets           # List budgets
PUT    /admin/budgets/:id       # Update budget
GET    /admin/usage             # Usage reports
```

---

## 6. Design Principles

### Speed

- Zero-copy streaming (never buffer full responses)
- Connection pooling with keep-alive
- Fast JSON parsing (sonic/go-json)
- Minimal allocations in hot path
- Async audit logging (non-blocking)
- Circuit breakers prevent cascade failures

### Quality

- Strict typing for all payloads
- Comprehensive error handling
- 100% test coverage on core paths
- Integration tests against real providers
- Load testing benchmarks

### Modularity

- Every feature is optional except core routing
- Plugin interfaces for all extensibility points
- No circular dependencies
- Feature flags for compile-time and runtime control
- Clean dependency injection

```go
type Features struct {
    Middleware    bool
    Guardrails    bool
    AuditLogging  bool
    AdminAPI      bool
    Failover      bool
    MultiTenant   bool
    RateLimiting  bool
    Caching       bool
}
```

---

## 7. Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| Core Provider Interface | ✅ Done | `internal/core/` |
| Provider Implementations | ✅ Done | `internal/providers/*/` |
| Model Registry | ✅ Done | `internal/providers/registry.go` |
| Router | ✅ Done | `internal/providers/router.go` |
| Cache (models) | ✅ Done | `internal/cache/` |
| Prometheus Metrics | ✅ Done | `internal/observability/` |
| HTTP Client | ✅ Done | `internal/llmclient/` |
| Middleware Chain | 🚧 Planned | `internal/middleware/` |
| Guardrails | 🚧 Planned | `internal/guardrails/` |
| Failover | 🚧 Planned | `internal/routing/failover.go` |
| Multi-key Support | 🚧 Planned | `internal/providers/keyring.go` |
| Pass-through | 🚧 Planned | `internal/server/passthrough.go` |
| Audit Logging | 🚧 Planned | `internal/audit/` |
| Admin API | 🚧 Planned | `internal/admin/` |
| Budget Management | 🚧 Planned | `internal/billing/` |

---

## 8. Why GOModel?

**vs LiteLLM:**

| Aspect | LiteLLM (Python) | GOModel (Go) |
|--------|------------------|--------------|
| Deployment | Python + pip | Single binary |
| Concurrency | asyncio | Goroutines (10k+ connections) |
| Memory | ~100MB+ | ~20MB |
| Type Safety | Runtime | Compile-time |
| Startup | Seconds | Milliseconds |

**Design Goals:**

1. **Drop-in replacement** - Same API, better performance
2. **Enterprise-ready** - Guardrails, audit, SSO
3. **Cloud-native** - Prometheus, OpenTelemetry, K8s-ready
4. **Operator-friendly** - Single binary, minimal config

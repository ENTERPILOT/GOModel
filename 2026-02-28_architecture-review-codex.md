# GoModel Architecture Review (Codex)

Date: 2026-02-28
Focus: deep architecture review with emphasis on `internal/providers`

## Findings (bugs/risks first)

1. **High: cached model routing can silently bind models to the wrong provider instance**
`LoadFromCache()` reconstructs provider mapping by `providerType` only, so multiple configured providers of the same type (for example two `openai` instances with different base URLs/keys) collapse to one provider on restore.

References:
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:194`
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:196`
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:203`
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:287`

2. **High: Responses stream converter can busy-loop CPU (`Read` returns `0, nil`)**
When no transformed bytes are ready, converter returns `0, nil`, which violates common `io.Reader` expectations and can cause hot loops in `io.Copy`/clients under timing edge cases.

References:
- `/Users/jakubaw/projects/gomodel/internal/providers/responses_converter.go:218`
- `/Users/jakubaw/projects/gomodel/internal/providers/responses_converter.go:219`

3. **High: `/v1/responses` compatibility is lossy for providers using `ResponsesViaChat`**
`ResponsesRequest` supports `tools`, `metadata`, `reasoning`, `stream_options`; conversion to `ChatRequest` drops most of these, so behavior differs by provider (native Responses providers vs chat-adapted providers).

References:
- `/Users/jakubaw/projects/gomodel/internal/core/responses.go:9`
- `/Users/jakubaw/projects/gomodel/internal/core/responses.go:13`
- `/Users/jakubaw/projects/gomodel/internal/core/responses.go:15`
- `/Users/jakubaw/projects/gomodel/internal/providers/responses_adapter.go:23`
- `/Users/jakubaw/projects/gomodel/internal/providers/responses_adapter.go:64`

4. **Medium: registry startup goroutine ignores parent cancellation**
`InitializeAsync` starts background init with `context.Background()` instead of deriving from caller context; shutdown/cancel won’t stop this task early.

Reference:
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:317`

5. **Medium: transport layer treats only HTTP 200 as success**
Both normal and streaming paths reject any non-200 status, which is brittle for APIs that can validly return other 2xx statuses.

References:
- `/Users/jakubaw/projects/gomodel/internal/llmclient/client.go:289`
- `/Users/jakubaw/projects/gomodel/internal/llmclient/client.go:399`

6. **Medium: streaming copy errors are swallowed**
`io.Copy` result is ignored, so upstream stream truncation/client disconnect/provider errors are invisible to HTTP error handling and observability.

Reference:
- `/Users/jakubaw/projects/gomodel/internal/server/handlers.go:79`

7. **Medium (architectural): model collisions are resolved by “first provider wins”**
Duplicate model IDs across providers are silently dropped, preventing explicit policy (priority, weighted, health-aware failover) at model level.

Reference:
- `/Users/jakubaw/projects/gomodel/internal/providers/registry.go:112`

## Architecture Improvement Proposals

1. Replace `model -> provider` with `model -> route set` (`[]ProviderInstance`) and routing policy (`priority`, `weighted`, `latency`, `health-aware`).
2. Introduce stable `provider_instance_id` everywhere (config, registry, cache, admin API) and stop using `providerType` as identity.
3. Split provider abstraction into capabilities: `ChatProvider`, `ResponsesProvider`, `ModelCatalogProvider`, `StreamingProvider`; remove silent adapter fallback unless explicitly configured.
4. Create a first-class protocol normalization layer for SSE/events (typed parser + serializer), shared by Anthropic/OpenAI-compatible converters.
5. Make startup deterministic: explicit warmup phases (`cache load`, `provider probe`, `model sync`, `ready`) and expose readiness state via `/health` detail.
6. Make llm transport status-class aware (`2xx success`), and add provider-specific acceptable-status overrides where needed.
7. Enforce strict request validation before provider call (required fields, incompatible params, unsupported combinations per provider capability).
8. Unify resilience policy at route level (retry/failover budget per request across providers, not only per provider HTTP client).
9. Expand provider test strategy: contract matrix per provider + adapter parity tests (`Responses native` vs `ResponsesViaChat`) + cache restore tests with multiple instances of same provider type.
10. Refactor `internal/providers` into concern-oriented subpackages: `catalog`, `routing`, `adapters`, `transport`, `providers/<name>` to reduce coupling.

## Validation Notes

- Static review was completed across architecture docs and key runtime packages (`internal/app`, `internal/server`, `internal/llmclient`, `internal/providers/*`).
- In this sandbox, full provider tests requiring `httptest` listeners cannot run due to local bind restrictions.

## Detailed Design Direction (Expanded)

### 1) Replace `model -> provider` with `model -> route set`

Current state:
- Registry stores a single provider per model and duplicate model IDs use first-wins behavior.
- This blocks explicit model-level failover or balancing strategy.

Target state:
- Store `model_id -> []RouteTarget`.
- `RouteTarget` should include at least:
  - `provider_instance_id`
  - `priority`
  - `weight`
  - optional constraints (`regions`, `max_rpm`, `capabilities`)
- Router should select through a pluggable policy engine:
  - `priority`
  - `weighted`
  - `latency`
  - `health-aware`

Why this matters:
- Enables explicit routing policy per model instead of implicit map behavior.
- Makes resilience and traffic engineering first-class.

Breaking impact:
- Router and registry contracts move from 1:1 to 1:N model mapping.

### 2) Introduce stable `provider_instance_id` everywhere

Current state:
- Cache restore associates models via `providerType` (taxonomy), not instance identity.
- Multiple instances of same type can collapse incorrectly.

Target state:
- Add immutable IDs in config (for example `openai_primary`, `openai_backup`).
- Persist `provider_instance_id` in cache payload and admin outputs.
- Include `provider_instance_id` in usage/audit records for precise attribution.
- Keep `provider_type` only as classification.

Why this matters:
- Prevents identity collisions.
- Enables correct debugging, billing, and routing analytics.

Breaking impact:
- Config schema and cache format need versioned migration.

### 3) Split provider abstraction by capabilities

Current state:
- One broad provider interface forces all providers to implement all methods.
- Adapters can silently fill gaps (`ResponsesViaChat`), hiding compatibility differences.

Target state:
- Introduce capability interfaces:
  - `ChatProvider`
  - `StreamingChatProvider`
  - `ResponsesProvider`
  - `StreamingResponsesProvider`
  - `ModelCatalogProvider`
- Registration records capability bitmap per provider instance.
- Adapter usage must be explicit in config (for example `responses_mode: adapter`).

Why this matters:
- Prevents silent behavioral divergence.
- Improves feature discoverability and safer routing decisions.

Breaking impact:
- Provider constructor/registration APIs change.

### 4) Build a first-class protocol normalization layer for SSE/events

Current state:
- Multiple converters reimplement SSE parsing/serialization differently.
- Edge cases (partial lines, no output-ready frames, event ordering) are hard to reason about globally.

Target state:
- New shared package with:
  - SSE frame parser (stream-safe, incremental)
  - typed internal events (`TextDelta`, `UsageDelta`, `Completed`, `Error`)
  - serializers for target protocols (OpenAI chat chunk / Responses API events)
- Provider adapters only map provider-native events into typed normalized events.

Why this matters:
- Eliminates duplicated parser logic.
- Makes streaming correctness and observability testable once.

Breaking impact:
- Existing provider stream converter implementations are replaced.

### 5) Make startup deterministic with explicit warmup phases

Current state:
- Startup is non-deterministic with async init and background refresh races.
- Service may accept traffic before model catalog state is reliable.

Target state:
- Lifecycle phases:
  - `config_loaded`
  - `providers_created`
  - `cache_loaded`
  - `provider_probed`
  - `model_sync_complete`
  - `ready`
- `/health` should return phase and degradation fields:
  - `ready`
  - `source` (`network`, `cache_only`)
  - `stale_seconds`
  - `failed_providers`

Why this matters:
- Predictable bootstrap and clearer operational signals.

Breaking impact:
- Readiness semantics and startup timing change.

### 6) Make transport status-class aware (`2xx`) with overrides

Current state:
- Transport treats only `200` as success.

Target state:
- Default success predicate: `status >= 200 && status < 300`.
- Add provider+endpoint override hook for strict endpoint handling where required.
- Keep retryability policy independent from success predicate.

Why this matters:
- Avoids brittle assumptions and reduces provider-specific branching.

Breaking impact:
- Certain responses previously considered failures may become successes.

### 7) Enforce strict request validation before provider call

Current state:
- Handler-level validation is minimal; unsupported parameters are often implicitly ignored downstream.

Target state:
- Add pre-routing validation layer:
  - required field validation
  - incompatible combinations
  - provider-capability checks per model/provider instance
- Emit deterministic error codes/messages for unsupported combinations.

Why this matters:
- Strong API contract and fewer hidden behavior differences.

Breaking impact:
- Some previously accepted requests become explicit `4xx` errors.

### 8) Unify resilience policy at route level

Current state:
- Retries/circuit breaker are mostly per-provider HTTP-client concerns.

Target state:
- Add request-scoped route resilience controller:
  - global attempt budget
  - cross-provider failover budget
  - idempotency-aware retries
  - latency/SLO-aware target switching
- Provider-level retry remains as low-level transport fallback, but route-level policy is authoritative.

Why this matters:
- Better end-to-end success control across multi-provider routes.

Breaking impact:
- Retry/failover behavior changes materially and must be documented.

### 9) Expand provider test strategy into a contract matrix

Current state:
- Mostly provider-local tests; cross-provider parity and adapter equivalence are under-tested.

Target state:
- Shared contract suite that every provider instance must pass:
  - chat
  - stream chat
  - responses
  - stream responses
  - cancellation
  - header propagation
  - usage extraction
  - error mapping
- Adapter parity tests:
  - compare `Responses native` vs `ResponsesViaChat` against declared compatibility profile
- Cache-restore tests with multiple instances of same provider type.

Why this matters:
- Prevents regressions in feature parity and routing correctness.

Breaking impact:
- Existing providers may fail initially until behavior is normalized or explicitly declared as partial.

### 10) Refactor `internal/providers` into concern-driven subpackages

Current state:
- Registry, routing, adapters, transport concerns are mixed in one package area, increasing coupling.

Target layout:
- `internal/providers/catalog`
- `internal/providers/routing`
- `internal/providers/adapters`
- `internal/providers/transport`
- `internal/providers/instances/<vendor>`

Why this matters:
- Clear ownership boundaries and lower change blast radius.
- Easier to evolve routing logic without touching vendor implementations.

Breaking impact:
- Broad import path and wiring changes across app initialization and tests.

## Live Runtime Bug Hunt (localhost:8080)

Date: 2026-02-28
Method: direct `curl` probing against running instance, using cheap models where possible.

### Test Setup

- Health check target: `http://localhost:8080/health`
- Model discovery: `http://localhost:8080/v1/models`
- Cheap models used for active tests:
  - `llama-3.1-8b-instant`
  - `gpt-4.1-nano`
- Discovered model count from `/v1/models`: `184`

### Confirmed Findings

1. **Critical contract bug: `tools` are silently ignored on adapter-backed `/v1/responses`**

- Adapter-backed request (`llama-3.1-8b-instant`) with `tools` returns plain text assistant output, no function call item.
- Native OpenAI request (`gpt-4.1-nano`) with equivalent `tools` returns `function_call` output item.
- This is a behavior divergence on the same public endpoint and request schema.

2. **Critical schema bug: streamed adapter `/v1/responses` usage payload shape is wrong**

- Adapter stream `response.completed` event includes usage keys like `prompt_tokens`/`completion_tokens` plus timing fields.
- Native Responses stream uses expected `input_tokens`/`output_tokens`.
- This breaks client assumptions for one endpoint.

3. **Validation bug: `/v1/responses` invalid `input` errors reference `messages`**

- Requests with `input: null`, `input: []`, or `input: 123` return errors like:
  - `'messages' : minimum number of items is 1`
- Error fielding is incorrect for Responses API.

4. **Validation bug: missing chat model gives misleading message**

- `POST /v1/chat/completions` without `model` returns:
  - `unsupported model: `
- Should be explicit required-field validation (`model is required`).

5. **Capability-filter bug: many `/v1/models` entries are unusable for chat/responses**

Examples observed:
- Chat on embedding model (`text-embedding-3-small`) -> upstream `401`.
- Chat on audio transcription model (`whisper-1`) -> upstream `404` non-chat model.
- Responses on moderation model (`omni-moderation-latest`) -> upstream `500 provider_error`.

The gateway advertises models without endpoint capability gating, causing runtime failures.

6. **Security issue: admin API exposed without authentication**

- `GET /admin/api/v1/models` returned `200` and full model/provider mapping.
- `GET /admin/api/v1/usage/summary?days=1` returned `200`.

7. **Security issue: Swagger UI exposed without authentication**

- `GET /swagger/index.html` returned `200`.

8. **HTTP semantics issue: unsupported media type mapped to 400**

- Sending `Content-Type: text/plain` returns payload containing `code=415` text.
- Actual HTTP status is `400 Bad Request` instead of `415 Unsupported Media Type`.

9. **Information leakage: internal bind/unmarshal details exposed to clients**

- Wrong `messages` type reveals internal parsing details (Go type names, field names, JSON unmarshal internals).
- Error verbosity should be reduced for public responses.

10. **Protocol consistency issue on `/v1/responses` streaming**

- Native OpenAI streams include rich canonical event sequence (`response.in_progress`, `response.output_item.*`, sequence numbers, etc.).
- Adapter-backed streams emit reduced custom event set (`response.created`, deltas, `response.completed`).
- Same endpoint has materially different stream protocol shape depending on provider path.

### Notes

- These findings are from live runtime probing, not static-only review.
- Upstream provider error content appeared in some responses, indicating gateway pass-through behavior that may need normalization.

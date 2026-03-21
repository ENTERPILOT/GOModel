# ADR-0006: Semantic Response Cache

## Status

Accepted

## Context

GOModel already has an exact-match response cache (`simpleCacheMiddleware`) that hashes the full request body and returns a stored response on byte-identical requests. This covers the trivial case but misses the large class of requests that express the same intent differently:

- "What's the capital of France?" vs. "Which city is France's capital?"
- "Explain quantum entanglement simply" vs. "ELI5 quantum entanglement"

Production data from comparable systems shows exact-match caching achieves ~18% hit rates, while semantic caching pushes this to ~67% — a meaningful reduction in LLM API costs and latency.

A semantic cache layer is needed that can recognize meaning-equivalent queries and serve a stored response without making an upstream LLM call.

## Decision

### Layer Position

Semantic caching is a **second layer behind exact-match caching**, not a replacement. The exact-match check (sub-millisecond) always runs first; semantic search (~50–100ms, including embedding) runs only on exact misses. Both stores are written asynchronously after a real LLM response, never blocking the response path.

```
Request
  → simpleCacheMiddleware (exact SHA-256 match)
    → HIT: return immediately (X-Cache: HIT exact)
    → MISS: semanticCacheMiddleware (vector KNN search)
        → HIT: return immediately (X-Cache: HIT semantic)
        → MISS: forward to LLM → store both → return to client
```

### Embedding

Two implementations behind a common `Embedder` interface:

- **`MiniLMEmbedder`** (default): runs `all-MiniLM-L6-v2` locally via ONNX Runtime — no external API call, no extra infrastructure. 384-dimensional vectors. Activated when `embedder.provider` is `"local"` or absent.
- **`APIEmbedder`**: calls `POST /v1/embeddings` on any OpenAI-compatible provider already configured in the `providers` map, reusing that provider's `api_key` and `base_url`. Activated when `embedder.provider` matches a named provider (e.g. `"openai"`, `"groq"`). Unknown provider name is a hard startup error — no silent fallback.

The local default is a deliberate differentiator: neither Bifrost nor LiteLLM offer zero-infrastructure semantic caching.

### Vector Store

A `VecStore` interface with a `Type`-switched factory mirrors the existing `StorageConfig` pattern. Backends:

| Type | Notes |
|------|-------|
| `sqlite-vec` (default) | Embedded, CGO-free, file-based. Zero extra services. |
| `qdrant` | External Qdrant service. First external backend. |
| `pgvector` | For users already running PostgreSQL. |

### Text Extraction

The text to embed is the **last user message** from the `messages` array (for `/v1/chat/completions`) or the last user input (for `/v1/responses`). This is GPTCache's pragmatic `last_content` default — it trades full-context accuracy for high hit rates. The full conversation is not embedded because embedding long concatenated conversations produces noisy vectors with rapidly degrading hit rates.

If `exclude_system_prompt` is true, system messages are stripped before both the message count check and the embedding step, since system prompts are often identical across requests and add noise without improving discrimination.

### Parameter Isolation (`params_hash`)

Semantically identical prompts with different output-shaping parameters must not share cache entries. A SHA-256 of `model + temperature + top_p + max_tokens + tools_hash + response_format + stream` is computed for each request and stored as metadata alongside the vector entry. All KNN searches filter by this hash. This is the most significant correctness gap in LiteLLM's implementation and is non-negotiable.

**Future hook**: when a guardrails/ExecutionPlan pipeline is added, a `guardrails_hash` should be appended to `params_hash`. The schema is designed to accommodate this without migration.

### Conversation History Threshold

Semantic caching is skipped when the number of non-system messages exceeds `max_conversation_messages` (default: 3). Long multi-turn conversations are poor cache candidates — each new turn changes the context, hit rates approach zero, and the embedding cost is never recovered. This matches Bifrost's `ConversationHistoryThreshold`. Exact-match caching still applies regardless.

### Similarity Threshold

Default: **0.92**. Bifrost defaults to 0.80 (too aggressive for correctness-sensitive use cases). LiteLLM sets no default. The 0.90–0.95 range is the industry consensus for balanced production deployments. Users should start at 0.92 and lower based on measured false positive rates for their domain.

### Per-Request Header Overrides

Following Bifrost's precedent, callers can override cache behavior per request:

- `X-Cache-Semantic-Threshold` — override similarity threshold for this request
- `X-Cache-TTL` — override TTL for this request
- `X-Cache-Type` — `exact`, `semantic`, or `both` (default)
- `X-Cache-Control: no-store` — skip all caching for this request

### What is Explicitly Not Implemented

- **Streaming response caching**: streaming requests are skipped entirely. Storing and replaying stream chunks faithfully is a significant additional complexity and is deferred.
- **Guardrails/ExecutionPlan hash**: reserved in the `params_hash` design but not computed until a guardrails pipeline exists.
- **Endpoint normalization across `/chat/completions`, `/responses`, and pass-through**: a canonical request normalizer is the right long-term direction but is a larger refactor than the caching work. For now, `endpoint_type` is included in `params_hash`, preventing cross-endpoint hits.
- **Cache pre-warming, eviction policies, manual purge API**: deferred.

## Consequences

### Positive

- Semantic hit rates of 60–70% achievable for high-repetition workloads (support bots, FAQ, classification pipelines), vs. ~18% for exact-match alone.
- Zero additional infrastructure in the default configuration (local MiniLM + sqlite-vec).
- Correctness is preserved: parameter isolation prevents serving creative responses for deterministic requests, and the conversation threshold avoids bad cache entries for long multi-turn sessions.
- API embedder reuses existing provider credentials — no new secrets to manage.
- `VecStore` and `Embedder` interfaces make backends swappable without touching middleware logic.

### Negative

- Adds ~50–100ms latency on cache miss (embedding generation + vector search) for requests that would have gone to the LLM anyway. This is acceptable since the LLM call itself is 500ms–5s.
- Local MiniLM requires `libonnxruntime` to be present at runtime. Deployment documentation must cover this.
- False positives are possible. The default threshold of 0.92 minimizes but does not eliminate them. Users in correctness-critical domains (healthcare, finance, structured output) should use 0.97+ or disable semantic caching.
- Semantic cache does not help for creative, real-time data, or highly personalized workloads — these should use `X-Cache-Control: no-store` or `X-Cache-Type: exact`.

## Alternatives Considered

### Use Redis with RediSearch as the vector store

Rejected as the default. Redis is an external dependency that many single-instance deployments do not want. sqlite-vec achieves the same goal with zero infrastructure. Redis/RediSearch can be added later as a fourth vector store backend if there is demand.

### Embed the full conversation history

Rejected. Produces noisy vectors for conversations beyond 2–3 messages, hit rates degrade silently, and embedding long contexts is expensive. `last_content` (last user message) is the pragmatic default used by GPTCache and yields the best hit-rate/accuracy tradeoff for gateway use cases.

### Use a single cache backend for both exact and semantic

Rejected. Exact-match and semantic caches have different storage requirements (key-value vs. vector + metadata), different TTL semantics, and different infrastructure choices. Keeping them separate allows each to be configured and scaled independently.

### Always require an external embedding API (Bifrost approach)

Rejected. Requiring an external embedding API for a cache feature introduces a circular dependency risk (the embedding call could itself be cached or fail) and adds a hard external dependency. The local MiniLM model is sufficient for similarity detection and keeps the zero-infrastructure promise.

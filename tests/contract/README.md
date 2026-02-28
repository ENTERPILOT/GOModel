# Contract Replay Tests

Contract tests in this project are **adapter replay tests**.

They do not call external APIs in CI. Instead, they replay recorded provider payloads from `testdata/` through the real provider adapters and verify the normalized `core` output.

## What is validated

- Real adapter parsing paths (`ChatCompletion`, `StreamChatCompletion`, `ListModels`, `Responses`, `StreamResponses`)
- Streaming conversion behavior (`[DONE]`, chunk/event mapping)
- Provider-specific conversion logic (for example Anthropic -> OpenAI-compatible / Responses output)

## How replay works

1. A custom in-memory `http.RoundTripper` returns recorded fixtures for expected method/path routes.
2. Provider adapters are constructed with `NewWithHTTPClient(...)` and pointed at a local replay base URL.
3. Tests call adapter methods directly and assert normalized outputs.

No sockets are opened and no network access is required.

## Fixture layout

```text
testdata/
├── openai/
├── anthropic/
├── gemini/
├── groq/
└── xai/
```

Each folder contains recorded JSON and SSE payloads used by replay tests.

## Running

```bash
# Run contract replay tests
go test -v -tags=contract ./tests/contract/...

# Make target
make test-contract
```

## Updating fixtures

Use `cmd/recordapi` (or provider curl calls) to refresh payloads when provider contracts change, then re-run the contract suite.

# GoModel - AI Gateway

## Purpose
GoModel is a high-performance AI gateway in Go that routes requests to multiple LLM providers through a consistent API.
It is an OpenAI-compatible API AI gateway.

## Core Principles
- Follow Postel's Law: be conservative in what you send, liberal in what you accept.
- Keep implementation explicit and maintainable over clever abstractions.
- Backward compatibility is not a primary constraint in the current development stage.

## Engineering Guidelines
- Register providers explicitly via `factory.Add()` in `cmd/gomodel/main.go`.
- Use typed gateway errors (`core.GatewayError`) and map upstream failures correctly.
- Keep routing and provider behavior predictable; ensure the model registry is initialized before routing.
- Do not log secrets; preserve sensitive-header redaction behavior in audit logs.
- Keep configuration behavior consistent: defaults -> YAML -> environment variables (env wins).

## Testing Expectations
- Add or update tests for any behavior change.
- Use the correct test layers:
  - Unit: `make test`
  - E2E: `make test-e2e`
  - Integration: `make test-integration`
  - Contract: `make test-contract` (record with `make record-api` when needed)

## Documentation Expectations
- Update docs whenever behavior, config, providers, commands, or public APIs change.
- Check all relevant layers:
  - `README.md` and related README files
  - Exported Go doc comments
  - `docs/` technical documentation

## Quick Workflow
1. Make small, focused changes.
2. Run format/lint/tests relevant to the change.
3. Update documentation that the change impacts.

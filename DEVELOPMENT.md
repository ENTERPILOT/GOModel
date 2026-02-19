# Development

## Testing

```bash
make test          # Unit tests
make test-e2e      # End-to-end tests (requires -tags=e2e; uses in-process mock servers, no Docker)
make test-all      # All tests
```

## Linting

Requires [golangci-lint](https://golangci-lint.run/welcome/install/).

```bash
make lint          # Check code quality
make lint-fix      # Auto-fix issues
```

## Log output

By default the server prints colorized, human-readable logs to stderr — useful for local development.

```
14:22:45 INFO  starting gomodel version=dev commit=none
14:22:45 WARN  SECURITY WARNING: GOMODEL_MASTER_KEY not set ...
14:22:45 INFO  starting server address=:8080
```

Set `LOG_FORMAT=json` to switch to structured JSON output, which is required for production log aggregators (CloudWatch, Datadog, etc.):

```bash
LOG_FORMAT=json make run
```

## Pre-commit

```bash
pip install pre-commit
pre-commit install
```

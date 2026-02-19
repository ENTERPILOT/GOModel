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

By default the server uses [tint](https://github.com/lmittmann/tint) to print colorized, human-readable logs to stderr — useful for local development. Colors are automatically disabled when stderr is not a TTY (e.g. when output is piped or redirected).

```text
12:12PM INFO  starting gomodel version=dev commit=none
12:12PM WARN  SECURITY WARNING: GOMODEL_MASTER_KEY not set ...
12:12PM INFO  starting server address=:8080
```

JSON is the default output format. Set `LOG_FORMAT=text` to switch to tinted human-readable output for local development:

```bash
LOG_FORMAT=text make run
```

## Pre-commit

```bash
pip install pre-commit
pre-commit install
```

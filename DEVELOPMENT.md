# Development

## Prerequisites

Install all required development tools in one step:

```bash
make install-tools
```

This installs:
- [golangci-lint v2](https://golangci-lint.run/welcome/install/) — required for `make lint`
- [pre-commit](https://pre-commit.com/) — required for git hook setup

After installing tools, set up the pre-commit hooks:

```bash
pre-commit install
```

## Testing

```bash
make test              # Unit tests
make test-e2e          # End-to-end tests (in-process mock servers, no Docker)
make test-integration  # Integration tests (requires Docker for testcontainers)
make test-contract     # Contract tests (golden file validation)
make test-all          # All tests (unit + e2e + integration + contract)
make record-api        # Record API responses for contract golden files
```

## Linting

Requires [golangci-lint v2](https://golangci-lint.run/welcome/install/)

```bash
make lint          # Check code quality
make lint-fix      # Auto-fix issues
```

## Log output

Log format is chosen automatically based on the environment:

- **TTY** (interactive terminal): colorized, human-readable text via [tint](https://github.com/lmittmann/tint)
- **Non-TTY** (piped, redirected, Docker, CI): structured JSON

```text
12:12PM INFO  starting gomodel version=dev commit=none
12:12PM WARN  SECURITY WARNING: GOMODEL_MASTER_KEY not set ...
12:12PM INFO  starting server address=:8080
```

Override the auto-detection with `LOG_FORMAT`:

| Value | Effect |
|---|---|
| _(unset)_ | Auto-detect: text+colors on TTY, JSON otherwise |
| `text` | Always text (no colors if not a TTY) |
| `json` | Always JSON, even on a TTY |

```bash
LOG_FORMAT=text make run   # force text output
LOG_FORMAT=json make run   # force JSON output
```

# Development

## Testing

```bash
make test          # Unit tests
make test-e2e      # End-to-end tests
make test-all      # All tests
```

## Linting

Requires [golangci-lint](https://golangci-lint.run/welcome/install/).

```bash
make lint          # Check code quality
make lint-fix      # Auto-fix issues
```

## Pre-commit

```bash
pip install pre-commit
pre-commit install
```

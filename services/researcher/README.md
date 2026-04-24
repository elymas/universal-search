# researcher

gpt-researcher wrapper service for Universal Search.

## Purpose

Wraps the [gpt-researcher](https://github.com/assafelovic/gpt-researcher) library to provide a
planner-executor-publisher pipeline as a sidecar to the Go orchestration plane.

## Development

```bash
# Install with uv (from repo root)
uv sync --package researcher

# Run tests
uv run --directory services/researcher pytest

# Lint
uv run --directory services/researcher ruff check .
```

## Environment Variables

See `.env.example` for required variables.

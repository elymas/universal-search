# storm

STORM long-form synthesis service for Universal Search.

## Purpose

Wraps the [knowledge-storm](https://github.com/stanford-oval/storm) library to produce
long-form reports with per-claim citations via the STORM pipeline.

## Development

```bash
# Install with uv (from repo root)
uv sync --package storm

# Run tests
uv run --directory services/storm pytest

# Lint
uv run --directory services/storm ruff check .
```

## Environment Variables

See `.env.example` for required variables.

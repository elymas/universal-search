# embedder

BGE-M3 embedding service for Universal Search.

## Purpose

Provides dense and sparse (SPLADE) embeddings using BGE-M3 for multilingual
hybrid retrieval (RRF fusion with Meilisearch BM25 and Qdrant dense vectors).

## Development

```bash
# Install with uv (from repo root)
uv sync --package embedder

# Run tests
uv run --directory services/embedder pytest

# Lint
uv run --directory services/embedder ruff check .
```

## Environment Variables

See `.env.example` for required variables.

# embedder

FastAPI sidecar service for dense and sparse embedding generation. Placeholder service — full BGE-M3 and FastEmbed integration lands in SPEC-IDX-002.

## Purpose

Exposes a REST API for generating embeddings used by the hybrid retrieval layer (Qdrant + Meilisearch). Currently a scaffold with only the `/health` endpoint implemented.

## Internal Port

`8000` — mapped externally per the docker-compose configuration in `deploy/docker-compose.yml`.

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `MODEL_CACHE_DIR` | No | Directory for caching downloaded embedding models (default: `./.cache/models`) |

Copy `.env.example` to `.env` and fill in values before running.

## Running Tests

```bash
uv run pytest
```

Or from workspace root:

```bash
.venv/bin/pytest services/embedder/tests/
```

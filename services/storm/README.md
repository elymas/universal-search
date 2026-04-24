# storm

FastAPI sidecar service wrapping [knowledge-storm](https://github.com/stanford-oval/storm) for long-form report generation with per-claim citations.

## Purpose

Exposes a REST API for triggering STORM-based deep research reports. The implementation is a scaffold; full report generation logic lands in SPEC-DEEP-001.

## Internal Port

`8000` — mapped externally per the docker-compose configuration in `deploy/docker-compose.yml`.

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | Yes | API key for OpenAI-compatible endpoint (STORM uses litellm internally) |

Copy `.env.example` to `.env` and fill in values before running.

## Running Tests

```bash
uv run pytest
```

Or from workspace root:

```bash
.venv/bin/pytest services/storm/tests/
```

# researcher

FastAPI sidecar service wrapping [gpt-researcher](https://github.com/assafelovic/gpt-researcher) for deep research synthesis.

## Purpose

Exposes a REST API for triggering and retrieving long-form research reports. The implementation is a scaffold; full synthesis logic lands in SPEC-SYN-001.

## Internal Port

`8000` — mapped externally per the docker-compose configuration in `deploy/docker-compose.yml`.

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | Yes (for synthesis) | API key for OpenAI-compatible endpoint |
| `TAVILY_API_KEY` | Yes (for search) | Tavily search API key used by gpt-researcher |
| `OPENAI_BASE_URL` | No | Override base URL for LiteLLM proxy |

Copy `.env.example` to `.env` and fill in values before running.

## Running Tests

```bash
uv run pytest
```

Or from workspace root:

```bash
.venv/bin/pytest services/researcher/tests/
```

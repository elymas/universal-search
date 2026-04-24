FROM python:3.11-slim AS base

WORKDIR /app

RUN pip install --no-cache-dir uv

COPY pyproject.toml .
COPY src/ src/

RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system --no-cache ".[dev]" || uv pip install --system --no-cache .

RUN groupadd --gid 1001 appgroup && \
    useradd --uid 1001 --gid appgroup --shell /bin/bash --create-home appuser

USER appuser

EXPOSE 8000

CMD ["uvicorn", "storm.main:app", "--host", "0.0.0.0", "--port", "8000"]

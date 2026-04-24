# syntax=docker/dockerfile:1
# Storm service — STORM long-form synthesis
# hadolint ignore=DL3008
FROM python:3.11-slim AS base

WORKDIR /app

# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip

COPY pyproject.toml ./
COPY src/ ./src/

# hadolint ignore=DL3013
RUN pip install --no-cache-dir .

RUN useradd --create-home --shell /bin/bash appuser
USER appuser

CMD ["python", "-m", "storm"]

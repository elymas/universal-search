# syntax=docker/dockerfile:1
# Storm service — STORM long-form synthesis
# hadolint ignore=DL3008
FROM python:3.11-slim AS base

WORKDIR /app

# Combine system setup into a single RUN to avoid DL3059.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip \
 && useradd --create-home --shell /bin/bash appuser

COPY pyproject.toml ./
COPY src/ ./src/

# hadolint ignore=DL3013
RUN pip install --no-cache-dir .

USER appuser

CMD ["python", "-m", "storm"]

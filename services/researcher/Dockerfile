# syntax=docker/dockerfile:1
# Researcher service — gpt-researcher wrapper
# hadolint ignore=DL3008
FROM python:3.11-slim AS base

WORKDIR /app

# Install dependencies via pip (uv not available in image by default)
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip

COPY pyproject.toml ./
COPY src/ ./src/

# hadolint ignore=DL3013
RUN pip install --no-cache-dir .

# Non-root user for security
RUN useradd --create-home --shell /bin/bash appuser
USER appuser

CMD ["python", "-m", "researcher"]

# syntax=docker/dockerfile:1
# Researcher service — synthesis sidecar (SPEC-SYN-001)
# Multi-stage build on python:3.11-slim with non-root user.
FROM python:3.11-slim AS base

WORKDIR /app

# Combine system setup into a single RUN to avoid DL3059.
# DL3008 (pin apt versions): apt-get install only installs curl for healthcheck;
# pinning is impractical across base image refreshes. DL3013: pip is bootstrap-pinned upstream.
# hadolint ignore=DL3008,DL3013
RUN pip install --no-cache-dir --upgrade pip \
 && apt-get update && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --shell /bin/bash appuser

COPY pyproject.toml ./
COPY src/ ./src/

# hadolint ignore=DL3013
RUN pip install --no-cache-dir .

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8081/health || exit 1

USER appuser

CMD ["python", "-m", "researcher"]

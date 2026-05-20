# syntax=docker/dockerfile:1
# Storm service — STORM long-form synthesis sidecar (SPEC-DEEP-001 M1)

# ---------- build stage ----------
FROM python:3.11-slim AS builder

WORKDIR /app

# Install uv for fast dependency resolution
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

COPY pyproject.toml ./
COPY src/ ./src/

# Install dependencies into a venv
RUN uv venv /app/.venv && \
    uv pip install --python /app/.venv/bin/python -e "."

# ---------- runtime stage ----------
FROM python:3.11-slim AS runtime

WORKDIR /app

# Create non-root user
RUN useradd --create-home --shell /bin/bash appuser

# Copy venv from builder
COPY --from=builder /app/.venv /app/.venv

# Copy application source
COPY src/ ./src/

# Set PATH to use venv
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONUNBUFFERED=1

USER appuser

EXPOSE 8001

CMD ["python", "-m", "storm"]

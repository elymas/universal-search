# syntax=docker/dockerfile:1
# Storm service — STORM long-form synthesis sidecar (SPEC-DEEP-001 M1)

# ---------- build stage ----------
FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca AS builder

WORKDIR /app

# Install uv for fast dependency resolution
COPY --from=ghcr.io/astral-sh/uv@sha256:d0a0a753ab981624b49c97abc98821c1c09f4ca69d1ef5cee69c501be3d88479 /uv /usr/local/bin/uv

COPY pyproject.toml ./
COPY src/ ./src/

# Install dependencies into a venv
RUN uv venv /app/.venv && \
    uv pip install --python /app/.venv/bin/python -e "."

# ---------- runtime stage ----------
FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca AS runtime

WORKDIR /app

# Create non-root user
RUN useradd --create-home --shell /bin/bash appuser

# Upgrade the base-image pip toolchain to clear FIXED HIGH CVEs in the bundled
# setuptools/wheel vendor tree (CVE-2026-23949, CVE-2026-24049). The app runs
# from /app/.venv, but trivy still scans the base system site-packages.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip setuptools wheel

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

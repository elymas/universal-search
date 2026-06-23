# Multi-stage Dockerfile for tokenizer-ko sidecar.
# SPEC-IDX-003: mecab-ko Python sidecar (FastAPI + uvicorn).
#
# Stage 1: build — install mecab-ko (requires build tools for C extensions).
# Stage 2: runtime — copy only the installed package and app source.

# --- Stage 1: builder ---
FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca AS builder

# Install system build dependencies for mecab-ko C extension.
# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy pyproject.toml and source. The src-layout package
# (packages.find where=["src"]) must be present before pip install, or the
# build backend fails to build the wheel.
COPY pyproject.toml ./
COPY src/ ./src/

# Install the package and its dependencies into a prefix directory.
RUN pip install --no-cache-dir --prefix=/install ".[dev]" 2>/dev/null || \
    pip install --no-cache-dir --prefix=/install .

# --- Stage 2: runtime ---
FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca AS runtime

# Runtime system deps: mecab-ko-dic dictionary is bundled in the wheel,
# but libstdc++ is needed for the C extension.
# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

# Upgrade the base-image pip toolchain to clear FIXED HIGH CVEs in the bundled
# setuptools/wheel vendor tree (CVE-2026-23949, CVE-2026-24049).
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip setuptools wheel

WORKDIR /app

# Copy the installed packages from the builder stage.
COPY --from=builder /install /usr/local

# Copy application source.
COPY src/ ./src/

# Add src to PYTHONPATH so tokenizer_ko is importable.
ENV PYTHONPATH=/app/src

# Non-root user for security.
RUN useradd -r -s /bin/false appuser
USER appuser

# Expose the sidecar port (configurable via TOKENIZER_KO_PORT).
EXPOSE 8083

# Healthcheck: poll /health every 15s, start after 10s.
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8083/health')" || exit 1

CMD ["python", "-m", "tokenizer_ko"]

# Multi-stage Dockerfile for tokenizer-ko sidecar.
# SPEC-IDX-003: mecab-ko Python sidecar (FastAPI + uvicorn).
#
# Stage 1: build — install mecab-ko (requires build tools for C extensions).
# Stage 2: runtime — copy only the installed package and app source.

# --- Stage 1: builder ---
FROM python:3.11-slim AS builder

# Install system build dependencies for mecab-ko C extension.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy pyproject.toml first for better layer caching.
COPY pyproject.toml ./

# Install the package and its dependencies into a prefix directory.
RUN pip install --no-cache-dir --prefix=/install ".[dev]" 2>/dev/null || \
    pip install --no-cache-dir --prefix=/install .

# --- Stage 2: runtime ---
FROM python:3.11-slim AS runtime

# Runtime system deps: mecab-ko-dic dictionary is bundled in the wheel,
# but libstdc++ is needed for the C extension.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

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

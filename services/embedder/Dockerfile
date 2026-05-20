# syntax=docker/dockerfile:1
# Embedder service — BGE-M3 embedding sidecar (SPEC-IDX-002)
# Multi-stage: deps layer cached separately from source for fast rebuilds.
# hadolint ignore=DL3008
FROM python:3.11-slim AS base

WORKDIR /app

# System setup: pip upgrade + non-root user in one layer.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip \
 && useradd --create-home --shell /bin/bash --uid 1001 appuser

# Dependency layer (invalidated only when pyproject.toml changes).
COPY pyproject.toml ./
# hadolint ignore=DL3013
RUN pip install --no-cache-dir ".[runtime]" \
 || pip install --no-cache-dir .

# Source layer.
COPY src/ ./src/

# Pre-download BGE-M3 model weights at build time when PREFETCH_MODEL=1.
# In CI leave unset; in production image builds set to 1.
ARG PREFETCH_MODEL=0
RUN if [ "$PREFETCH_MODEL" = "1" ]; then \
      python -c "from FlagEmbedding import BGEM3FlagModel; BGEM3FlagModel('BAAI/bge-m3', use_fp16=True)"; \
    fi

EXPOSE 8082

# Health check: poll /health until model is loaded (allow 120s startup).
HEALTHCHECK --interval=10s --timeout=5s --start-period=120s --retries=12 \
  CMD python -c "import urllib.request, sys; r=urllib.request.urlopen('http://localhost:8082/health'); sys.exit(0 if r.status==200 else 1)"

USER appuser

CMD ["python", "-m", "embedder"]

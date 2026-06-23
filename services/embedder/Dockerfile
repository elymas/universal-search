# syntax=docker/dockerfile:1
# Embedder service — BGE-M3 embedding sidecar (SPEC-IDX-002)
# Multi-stage: deps layer cached separately from source for fast rebuilds.
# hadolint ignore=DL3008
FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca AS base

WORKDIR /app

# System setup: pip upgrade + non-root user in one layer.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip setuptools wheel \
 && useradd --create-home --shell /bin/bash --uid 1001 appuser

# Build tools for C-extension deps. FlagEmbedding pulls zlib-state, which has
# no aarch64 wheel and compiles from sdist (needs gcc + zlib headers).
# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*

# Dependency + source layer. The src-layout package (packages.find where=["src"])
# must be present before pip install, or the build backend fails to build the wheel.
COPY pyproject.toml ./
COPY src/ ./src/
# Install CPU-only torch first. The default torch wheel (incl. linux/arm64)
# declares NVIDIA CUDA dependencies (~3GB of nvidia-* packages) gated on
# platform_system == "Linux". This image always runs CPU inference
# (EMBEDDER_DEVICE=cpu); the CUDA path is the opt-in docker-compose.gpu.yml
# override. Pinning the CPU index keeps the image lean and the build fast.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu \
 && pip install --no-cache-dir .

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

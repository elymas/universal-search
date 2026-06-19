# Multi-stage Dockerfile for YouTube extraction sidecar.
# SPEC-ADP-005a: yt-dlp FastAPI sidecar (port 8084).
#
# Stage 1: build — install Python deps into prefix directory.
# Stage 2: runtime — copy only installed packages and app source.

# --- Stage 1: builder ---
FROM python:3.11-slim AS builder

WORKDIR /build

# Upgrading pip itself to the latest resolver; pinning pip's own version adds
# churn without benefit, so DL3013 (pin versions in pip) is ignored here.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip

# Copy pyproject.toml and source. The src-layout package
# (packages = ["src/youtube_extract"]) must be present before pip install,
# or the build backend fails to build the wheel.
COPY pyproject.toml ./
COPY src/ ./src/

# Install the package and its dependencies into a prefix directory.
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --prefix=/install .

# --- Stage 2: runtime ---
FROM python:3.11-slim AS runtime

WORKDIR /app

# Copy the installed packages from the builder stage.
COPY --from=builder /install /usr/local

# Copy application source.
COPY src/ ./src/

# Add src to PYTHONPATH so youtube_extract is importable.
ENV PYTHONPATH=/app/src

# Non-root user for security (NFR-ADP5a-002).
RUN useradd -r -s /bin/false appuser
USER appuser

# Expose the sidecar port (configurable via YT_EXTRACT_PORT).
EXPOSE 8084

# Healthcheck: poll /health every 15s (NFR-ADP5a-004).
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=5 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8084/health')" || exit 1

CMD ["python", "-m", "youtube_extract"]

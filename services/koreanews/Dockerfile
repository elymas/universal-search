FROM python:3.11-slim@sha256:ae52c5bef62a6bdd42cd1e8dffef86b9cd284bde9427da79839de7a4b983e7ca

WORKDIR /app

COPY pyproject.toml .
# Upgrade the base-image pip toolchain first to clear FIXED HIGH CVEs in the
# bundled setuptools/wheel vendor tree (CVE-2026-23949, CVE-2026-24049).
# hadolint ignore=DL3013
RUN pip install --no-cache-dir --upgrade pip setuptools wheel \
 && pip install --no-cache-dir ".[test]"

COPY src/ ./src/

# Non-root user for security (closes trivy DS002 run-as-root IaC finding).
RUN useradd --create-home --shell /bin/bash appuser
USER appuser

EXPOSE 8002

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8002"]

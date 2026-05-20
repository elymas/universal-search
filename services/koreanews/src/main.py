"""
KoreaNewsCrawler (KNC) sidecar — HTTP stub (v0.1).

SPEC-ADP-009 REQ-ADP9-009: This is a scaffold returning HTTP 503 for all /search
requests. Full implementation is deferred to SPEC-ADP-009-KNC.

Run:
    uvicorn main:app --host 0.0.0.0 --port 8002
"""

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

app = FastAPI(title="KoreaNewsCrawler Sidecar", version="0.1.0-stub")


@app.get("/health")
async def health() -> dict:
    """Liveness probe — returns 200 even in stub mode."""
    return {"status": "stub", "version": "0.1.0-stub"}


@app.post("/search")
async def search(request: Request) -> JSONResponse:
    """
    Search endpoint stub — always returns HTTP 503.

    Full implementation requires SPEC-ADP-009-KNC including:
      - Korean web crawling with robots.txt compliance
      - GDPR/PIPA-compliant content handling
      - Rate limiting per upstream source
      - Result normalization to the KNC JSON schema
    """
    return JSONResponse(
        status_code=503,
        content={"detail": "knc sidecar not yet implemented (see SPEC-ADP-009-KNC)"},
    )

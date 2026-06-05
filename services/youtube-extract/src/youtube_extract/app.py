"""FastAPI application — YouTube extraction sidecar.

REQ-ADP5a-001: FastAPI sidecar with /health and /search endpoints.
REQ-ADP5a-002: GET /health returns 200 {"status":"ok",...} when ready.
REQ-ADP5a-003: POST /search returns items envelope matching Go adapter contract.
REQ-ADP5a-006: Error responses use structured envelope with category mapping.
"""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import AsyncGenerator, Optional

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from .models import (
    ErrorDetail,
    ErrorResponse,
    HealthResponse,
    SearchRequest,
    SearchResponse,
    YTItem,
)
from .ytdlp_runner import YtdlpError, get_ytdlp_version, run_search

logger = logging.getLogger("youtube_extract")

# ---------------------------------------------------------------------------
# Application state
# ---------------------------------------------------------------------------

_ready: bool = False
_ytdlp_version: str = "unknown"


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Probe yt-dlp at startup; set ready flag."""
    global _ready, _ytdlp_version

    _ytdlp_version = get_ytdlp_version()
    logger.info("sidecar.startup", extra={"ytdlp_version": _ytdlp_version})
    _ready = True

    yield

    _ready = False


app = FastAPI(title="YouTube Extract", lifespan=lifespan)


# ---------------------------------------------------------------------------
# Health endpoint (REQ-ADP5a-002)
# ---------------------------------------------------------------------------


@app.get("/health")
async def health() -> JSONResponse:
    """Return 200 when ready; 503 while loading.

    Go adapter requires status == "ok" exactly (youtube.go:142).
    """
    if not _ready:
        return JSONResponse(
            status_code=503,
            content={"status": "loading", "reason": "yt-dlp not ready"},
        )
    return JSONResponse(
        status_code=200,
        content={"status": "ok", "ytdlp_version": _ytdlp_version},
    )


# ---------------------------------------------------------------------------
# Search endpoint (REQ-ADP5a-003)
# ---------------------------------------------------------------------------


@app.post("/search")
async def search(req: SearchRequest) -> JSONResponse:
    """Execute YouTube search via yt-dlp subprocess.

    Returns the items envelope matching Go ytSearchResponse contract.
    """
    # REQ-ADP5a-006: validate required fields (Pydantic handles type validation).
    # Additional business validation.
    if not req.query or not req.query.strip():
        return _error_response(400, "permanent", "query is required and must not be empty")

    if not _ready:
        return _error_response(503, "unavailable", "sidecar not ready")

    try:
        items, has_more = await run_search(
            query=req.query,
            max_results=req.max_results,
            cursor_offset=req.cursor_offset,
            include_transcripts=req.include_transcripts,
            transcript_lang=req.transcript_lang,
            since=req.since,
        )
    except YtdlpError as exc:
        headers = {}
        if exc.retry_after > 0:
            headers["Retry-After"] = str(exc.retry_after)
        return _error_response(
            exc.http_status,
            exc.category,
            exc.message,
            headers=headers if headers else None,
        )
    except Exception as exc:
        logger.exception("search.unexpected_error")
        return _error_response(500, "unavailable", f"internal error: {exc}")

    response = SearchResponse(items=items, has_more=has_more)
    return JSONResponse(status_code=200, content=response.model_dump())


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _error_response(
    status: int,
    category: str,
    message: str,
    *,
    reason: Optional[str] = None,
    headers: Optional[dict[str, str]] = None,
) -> JSONResponse:
    """Build a structured error response matching the Go error envelope contract."""
    error_detail = ErrorDetail(category=category, message=message, reason=reason)
    body = ErrorResponse(error=error_detail)
    return JSONResponse(status_code=status, content=body.model_dump(), headers=headers)

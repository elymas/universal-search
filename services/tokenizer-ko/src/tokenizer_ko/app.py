"""FastAPI application for the tokenizer-ko sidecar.

REQ-IDX-003-001: POST /tokenize + GET /health endpoints.
REQ-IDX-003-003: Lifespan raises if mecab-ko-dic unavailable.
REQ-IDX-003-004: Empty/oversize input returns 400.

# @MX:NOTE: [AUTO] Sidecar entry point; wraps pymecab-ko with FastAPI lifespan.
# Single-process, single-worker (asyncio.Lock serialises Tagger calls).
# @MX:SPEC: SPEC-IDX-003
"""

from __future__ import annotations

import logging
import os
from contextlib import asynccontextmanager
from typing import AsyncGenerator

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from tokenizer_ko import __version__
from tokenizer_ko.models import TokenizeRequest, TokenizeResponse
from tokenizer_ko.obs import (
    OUTCOME_ERROR_INTERNAL,
    OUTCOME_SUCCESS,
    Timer,
    log_invalid_input,
    log_tokenize,
    setup_logging,
)
from tokenizer_ko.tokenize import (
    MAX_INPUT_BYTES,
    create_tagger,
    get_dict_version,
    tokenize_text,
)

logger = logging.getLogger("tokenizer_ko.app")

# Module-level dict_version set during lifespan startup.
_dict_version: str = "unknown"


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """FastAPI lifespan: initialise mecab-ko Tagger at startup.

    REQ-IDX-003-003: If Tagger construction fails, the exception propagates
    out of lifespan before the app accepts any requests.
    """
    log_level = os.getenv("TOKENIZER_KO_LOG_LEVEL", "INFO")
    setup_logging(log_level)

    logger.info("tokenizer-ko starting — loading mecab-ko Tagger")

    # This RAISES if mecab-ko-dic is unavailable (REQ-IDX-003-003).
    tagger = create_tagger()
    dict_ver = get_dict_version(tagger)

    # Store on app state for access by route handlers.
    app.state.tagger = tagger
    app.state.dict_version = dict_ver

    logger.info(
        "tokenizer-ko ready: dict_version=%s version=%s",
        dict_ver,
        __version__,
    )

    yield

    logger.info("tokenizer-ko shutting down")
    app.state.tagger = None


app = FastAPI(title="tokenizer-ko", version=__version__, lifespan=lifespan)


@app.get("/health")
async def health(request: Request) -> JSONResponse:
    """GET /health — returns service status.

    REQ-IDX-003-001 (§2.1 d): Returns 200 when healthy, 503 when Tagger failed.
    """
    tagger = getattr(request.app.state, "tagger", None)
    if tagger is None:
        return JSONResponse(
            status_code=503,
            content={
                "status": "degraded",
                "reason": "tagger not initialized",
            },
        )
    dict_ver = getattr(request.app.state, "dict_version", "unknown")
    return JSONResponse(
        content={
            "status": "ok",
            "version": __version__,
            "dict_version": dict_ver,
            "tokenizer": "mecab-ko",
        }
    )


@app.post("/tokenize", response_model=TokenizeResponse)
async def tokenize_endpoint(
    req: TokenizeRequest,
    request: Request,
) -> JSONResponse:
    """POST /tokenize — tokenise Korean text using mecab-ko.

    REQ-IDX-003-001: Accepts TokenizeRequest, returns TokenizeResponse.
    REQ-IDX-003-002: mecab-ko Tagger.parse with asyncio.Lock serialisation.
    REQ-IDX-003-004: 400 on empty or oversize text.
    REQ-IDX-003-009: Per-call structured log.
    """
    # REQ-IDX-003-004: validate non-empty (Pydantic str_strip_whitespace already applied).
    if not req.text:
        log_invalid_input(request_id=req.request_id, error="empty text")
        return JSONResponse(
            status_code=400,
            content={"error": "invalid_input", "detail": "text"},
        )

    # REQ-IDX-003-004: validate max size.
    if len(req.text.encode("utf-8")) > MAX_INPUT_BYTES:
        log_invalid_input(request_id=req.request_id, error="text too large")
        return JSONResponse(
            status_code=400,
            content={"error": "invalid_input", "detail": "size"},
        )

    tagger = getattr(request.app.state, "tagger", None)
    if tagger is None:
        return JSONResponse(
            status_code=503,
            content={"error": "service_unavailable", "detail": "tagger not ready"},
        )

    dict_ver = getattr(request.app.state, "dict_version", "unknown")

    try:
        with Timer() as timer:
            result = await tokenize_text(req.text, tagger)
        # Override latency with Timer (more accurate end-to-end).
        latency_ms = timer.elapsed_ms
        result["latency_ms"] = latency_ms
    except Exception as exc:
        logger.error(  # nosemgrep: python.lang.security.audit.logging.logger-credential-leak -- logs request_id (UUID) + exception text only, no secret
            "tokenize: internal error request_id=%s: %s",
            req.request_id,
            exc,
            exc_info=True,
        )
        log_tokenize(
            request_id=req.request_id,
            text_len=len(req.text),
            morpheme_count=0,
            latency_ms=0.0,
            outcome=OUTCOME_ERROR_INTERNAL,
        )
        return JSONResponse(
            status_code=500,
            content={"error": "internal_error", "detail": str(exc)},
        )

    log_tokenize(
        request_id=req.request_id,
        text_len=len(req.text),
        morpheme_count=result["morpheme_count"],
        latency_ms=result["latency_ms"],
        outcome=OUTCOME_SUCCESS,
    )

    return JSONResponse(
        content={
            "request_id": req.request_id,
            "tokens": result["tokens"],
            "joined": result["joined"],
            "morpheme_count": result["morpheme_count"],
            "latency_ms": result["latency_ms"],
            "dict_version": dict_ver,
        }
    )

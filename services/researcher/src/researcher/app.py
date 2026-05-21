"""FastAPI application for the researcher service.

REQ-SYN-001: POST /synthesize + GET /health endpoints.
REQ-SYN-004: Empty input validation (400 on empty query or zero docs).
"""

from __future__ import annotations

import logging
import os
from contextlib import asynccontextmanager
from typing import Any, AsyncGenerator

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from researcher.deep_tree import router as deep_tree_router
from researcher.faithfulness_endpoint import router as faithfulness_router
from researcher.gateway import Gateway
from researcher.models import SynthesizeRequest, SynthesizeResponse
from researcher.obs import log_synthesis, setup_logging
from researcher.synthesis import synthesize

logger = logging.getLogger(__name__)

__version__ = "0.1.0"


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Application lifespan: setup logging on startup."""
    setup_logging()
    logger.info({"message": "researcher service starting", "version": __version__})
    yield
    logger.info({"message": "researcher service shutting down"})


app = FastAPI(title="researcher", version=__version__, lifespan=lifespan)

# SPEC-DEEP-002 REQ-DEEP2-006: Faithfulness check endpoint.
app.include_router(faithfulness_router)

# SPEC-DEEP-003 Phase C: Tree decomposition endpoint.
app.include_router(deep_tree_router)


@app.get("/health")
async def health() -> dict[str, str]:
    """GET /health — returns service status.

    Returns 200 {status: ok} when healthy.
    REQ-SYN-001.
    """
    return {"status": "ok", "version": __version__}


@app.post("/synthesize", response_model=SynthesizeResponse)
async def synthesize_endpoint(req: SynthesizeRequest) -> SynthesizeResponse:
    """POST /synthesize — synthesize a paragraph from pre-fetched docs.

    REQ-SYN-001: Accepts SynthesizeRequest, returns SynthesizeResponse.
    REQ-SYN-004: Returns 400 for empty query or zero docs.
    REQ-SYN-006: Per-call observability via log_synthesis.
    """
    # REQ-SYN-004: Validate non-empty inputs (str_strip_whitespace already applied by Pydantic)
    if not req.query:
        logger.warning({
            "request_id": req.request_id,
            "error": "empty_input",
            "detail": "query",
        })
        return JSONResponse(
            status_code=400,
            content={"error": "empty_input", "detail": "query"},
        )

    if not req.docs:
        logger.warning({
            "request_id": req.request_id,
            "error": "empty_input",
            "detail": "docs",
        })
        return JSONResponse(
            status_code=400,
            content={"error": "empty_input", "detail": "docs"},
        )

    gateway = Gateway()
    return await synthesize(req, gateway)


@app.exception_handler(Exception)
async def generic_exception_handler(request: Request, exc: Exception) -> JSONResponse:
    """Catch-all exception handler to prevent 500 leaking stack traces."""
    logger.error({"message": "Unhandled exception", "error": str(exc)})
    return JSONResponse(
        status_code=500,
        content={"error": "internal_error", "detail": str(exc)},
    )

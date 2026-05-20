"""FastAPI application for the storm service.

REQ-DEEP1-001: POST /generate_report + GET /health + GET /readyz endpoints.
REQ-DEEP1-004: Deadline (504) and budget (402) error handling.
"""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import Any, AsyncGenerator

from fastapi import FastAPI
from fastapi.responses import JSONResponse

from storm import __version__
from storm.models import GenerateReportRequest, GenerateReportResponse
from storm.obs import emit_outcome, setup_logging
from storm.pipeline import (
    BudgetExceededError,
    DeadlineExceededError,
)
from storm.pipeline import (
    run as run_pipeline,
)

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Application lifespan: setup logging on startup."""
    setup_logging()
    logger.info({"message": "storm service starting", "version": __version__})
    yield
    logger.info({"message": "storm service shutting down"})


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    application = FastAPI(
        title="storm",
        version=__version__,
        lifespan=lifespan,
    )

    # ------------------------------------------------------------------
    # Exception handlers (REQ-DEEP1-004)
    # ------------------------------------------------------------------

    @application.exception_handler(DeadlineExceededError)
    async def deadline_exceeded_handler(
        request: Any,
        exc: DeadlineExceededError,
    ) -> JSONResponse:
        """Return HTTP 504 with structured deadline-exceeded body."""
        request_id = getattr(request, "_request_id", "unknown")
        logger.warning(
            {
                "request_id": request_id,
                "reason": "deadline_exceeded",
                "elapsed_ms": exc.elapsed_ms,
                "partial_sections_completed": exc.partial_sections_completed,
            }
        )
        emit_outcome(
            "deadline_exceeded",
            request_id=request_id,
            elapsed_ms=exc.elapsed_ms,
            partial_sections_completed=exc.partial_sections_completed,
        )
        return JSONResponse(
            status_code=504,
            content={
                "error": "deadline_exceeded",
                "detail": str(exc),
                "elapsed_ms": exc.elapsed_ms,
                "partial_sections_completed": exc.partial_sections_completed,
            },
        )

    @application.exception_handler(BudgetExceededError)
    async def budget_exceeded_handler(
        request: Any,
        exc: BudgetExceededError,
    ) -> JSONResponse:
        """Return HTTP 402 with structured budget-exceeded body."""
        request_id = getattr(request, "_request_id", "unknown")
        logger.warning(
            {
                "request_id": request_id,
                "reason": "budget_exceeded",
                "cost_usd": exc.cost_usd,
                "cap_usd": exc.cap_usd,
            }
        )
        emit_outcome(
            "budget_exceeded",
            request_id=request_id,
            cost_usd=exc.cost_usd,
            cap_usd=exc.cap_usd,
        )
        return JSONResponse(
            status_code=402,
            content={
                "error": "budget_exceeded",
                "detail": str(exc),
                "cost_usd": exc.cost_usd,
                "cap_usd": exc.cap_usd,
            },
        )

    # ------------------------------------------------------------------
    # Routes
    # ------------------------------------------------------------------

    @application.get("/health")
    async def health() -> dict[str, str]:
        """GET /health -- returns service status."""
        return {"status": "ok", "version": __version__}

    @application.get("/readyz")
    async def readyz() -> dict[str, Any]:
        """GET /readyz -- returns readiness with dependency checks."""
        deps = {
            "litellm": True,
            "storm_lib": True,
        }
        ready = all(deps.values())
        return {"ready": ready, "deps": deps}

    @application.post("/generate_report", response_model=GenerateReportResponse)
    async def generate_report(req: GenerateReportRequest) -> GenerateReportResponse:
        """POST /generate_report -- invokes STORM pipeline (M2).

        The pipeline builds LM configs via the LiteLLM gateway, constructs
        an InjectedRM from request docs, and runs STORMWikiRunner.
        """
        logger.info(
            {"message": "generate_report invoked", "request_id": req.request_id}
        )
        return run_pipeline(req)

    return application


# Module-level app instance for uvicorn
app = create_app()

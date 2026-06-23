"""FastAPI application — BGE-M3 embedding sidecar.

REQ-IDX-002-001: POST /embed endpoint.
REQ-IDX-002-003: GET /health endpoint; 503 while loading, 200 after ready.
REQ-IDX-002-004: LRU cache integration.
REQ-IDX-002-006: Per-call observability.
REQ-IDX-002-012: Model loaded once in lifespan startup.
"""

from __future__ import annotations

import os
import time
from contextlib import asynccontextmanager
from typing import Any, AsyncGenerator, Optional

from fastapi import FastAPI
from fastapi.responses import JSONResponse

from .cache import EmbedderCache
from .embed import Embedder, EmbedValidationError
from .models import EmbedRequest, EmbedResponse
from .obs import log_embed, logger, timer

# ---------------------------------------------------------------------------
# Configuration from environment variables (OWASP: avoid hardcoded defaults)
# ---------------------------------------------------------------------------
_MODEL_NAME = os.getenv("EMBEDDER_MODEL", "BAAI/bge-m3")
_MODEL_VERSION = os.getenv("EMBEDDER_MODEL_VERSION", "latest")
_DEVICE = os.getenv("EMBEDDER_DEVICE", "cpu")
_USE_FP16 = os.getenv("EMBEDDER_USE_FP16", "false").lower() == "true"
_MAX_LENGTH = int(os.getenv("EMBEDDER_MAX_LENGTH", "8192"))
_CACHE_MAX_ENTRIES = int(os.getenv("EMBEDDER_CACHE_MAX_ENTRIES", "10000"))

# ---------------------------------------------------------------------------
# Application state (module-level singletons set during lifespan)
# ---------------------------------------------------------------------------
_embedder: Optional[Embedder] = None
_cache: Optional[EmbedderCache] = None
_model_ready: bool = False


# @MX:NOTE: [AUTO] lifespan controls model load/free; do not add per-request init.
@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Load BGE-M3 model at startup; free it at shutdown."""
    global _embedder, _cache, _model_ready

    logger.info(
        "embedder.loading",
        extra={
            "model": _MODEL_NAME,
            "model_version": _MODEL_VERSION,
            "device": _DEVICE,
            "use_fp16": _USE_FP16,
        },
    )

    load_start = time.perf_counter()
    _embedder = Embedder(
        model_name=_MODEL_NAME,
        device=_DEVICE,
        use_fp16=_USE_FP16,
        max_length=_MAX_LENGTH,
        model_version=_MODEL_VERSION if _MODEL_VERSION != "latest" else None,
    )
    _cache = EmbedderCache(maxsize=_CACHE_MAX_ENTRIES)
    load_seconds = time.perf_counter() - load_start
    _model_ready = True

    logger.info(
        "embedder.model_loaded",
        extra={
            "model": _MODEL_NAME,
            "model_version": _embedder.model_version,
            "device": _DEVICE,
            "use_fp16": _USE_FP16,
            "load_seconds": round(load_seconds, 3),
        },
    )
    # Single INFO record marking the service as ready (REQ-IDX-002-003).
    logger.info("embedder.ready", extra={"model": _MODEL_NAME})

    yield

    # Shutdown: release model reference.
    _model_ready = False
    if _embedder is not None:
        _embedder.free()
        _embedder = None
    _cache = None


app = FastAPI(title="Embedder", lifespan=lifespan)


# ---------------------------------------------------------------------------
# Health endpoint (REQ-IDX-002-003)
# ---------------------------------------------------------------------------
@app.get("/health")
async def health() -> JSONResponse:
    """Return 200 when ready; 503 while loading."""
    if not _model_ready or _embedder is None:
        return JSONResponse(
            status_code=503,
            content={"status": "loading", "reason": "model not ready"},
        )
    return JSONResponse(
        status_code=200,
        content={
            "status": "ok",
            "model": _embedder.model_name,
            "model_version": _embedder.model_version,
            "device": _embedder.device,
        },
    )


# ---------------------------------------------------------------------------
# Embed endpoint
# @MX:ANCHOR: [AUTO] Primary embedding entry point; callers: SPEC-IDX-001, tests, Go client
# @MX:REASON: fan_in >= 3; all embedding requests route through this function
# ---------------------------------------------------------------------------
@app.post("/embed")
async def embed(req: EmbedRequest) -> JSONResponse:
    """Embed texts using BGE-M3.

    Returns HTTP 503 while model is loading.
    Returns HTTP 400 for invalid inputs.
    Returns HTTP 500 on OOM.
    """
    if not _model_ready or _embedder is None or _cache is None:
        return JSONResponse(
            status_code=503,
            content={
                "error": "model_loading",
                "detail": "model is still initialising; retry shortly",
            },
        )

    outcome = "error_internal"
    cache_hits = 0
    cache_misses = 0

    with timer() as t:
        try:
            result = await _run_embed(req)
            cache_hits = result["cache_hits"]
            cache_misses = result["cache_misses"]
            outcome = "success"
        except EmbedValidationError as exc:
            outcome = "error_invalid"
            logger.warning(
                "embed.invalid",
                extra={"request_id": req.request_id, "error": exc.code},
            )
            return JSONResponse(
                status_code=400,
                content={"error": exc.code, "detail": exc.detail},
            )
        except MemoryError as exc:
            outcome = "error_oom"
            logger.error(  # nosemgrep: python.lang.security.audit.logging.logger-credential-leak -- logs request_id + exception class name only, no secret
                "embed.oom",
                extra={
                    "request_id": req.request_id,
                    "exception_class": type(exc).__name__,
                },
            )
            return JSONResponse(
                status_code=500,
                content={
                    "error": "oom",
                    "detail": "inference out of memory; retry with smaller batch_size",
                },
            )
        except Exception as exc:
            outcome = "error_internal"
            logger.error(  # nosemgrep: python.lang.security.audit.logging.logger-credential-leak -- logs request_id + exception class name only, no secret
                "embed.error",
                extra={
                    "request_id": req.request_id,
                    "exception_class": type(exc).__name__,
                },
            )
            return JSONResponse(
                status_code=500,
                content={"error": "internal", "detail": str(exc)},
            )

    log_embed(
        request_id=req.request_id,
        texts_count=len(req.texts),
        return_dense=req.return_dense,
        return_sparse=req.return_sparse,
        return_colbert_vecs=req.return_colbert_vecs,
        cache_hits=cache_hits,
        cache_misses=cache_misses,
        latency_ms=t.get("latency_ms", 0.0),
        model=_embedder.model_name,
        model_version=_embedder.model_version,
        device=_embedder.device,
        outcome=outcome,
    )

    return JSONResponse(
        status_code=200,
        content=EmbedResponse(
            request_id=req.request_id,
            dense=result["dense"],
            sparse=result["sparse"],
            colbert=result["colbert"],
            model=_embedder.model_name,
            model_version=_embedder.model_version,
            device=_embedder.device,
            latency_ms=round(t.get("latency_ms", 0.0), 3),
            cache_hits=cache_hits,
            cache_misses=cache_misses,
        ).model_dump(),
    )


async def _run_embed(req: EmbedRequest) -> dict[str, Any]:
    """Coordinate cache lookup + inference, preserving request order.

    Returns dict with keys: dense, sparse, colbert, cache_hits, cache_misses.
    """
    assert _embedder is not None
    assert _cache is not None

    texts = req.texts
    # Validation is delegated to Embedder._validate (raises EmbedValidationError).
    _embedder._validate(texts, req.return_dense, req.return_sparse, req.return_colbert_vecs)  # type: ignore[attr-defined]

    n = len(texts)
    cache_hits = 0
    cache_misses = 0

    hits: dict[int, tuple] = {}
    miss_indices: list[int] = []

    for i, text in enumerate(texts):
        key = _cache.key_for(
            text,
            _embedder.model_name,
            _embedder.model_version,
            req.return_dense,
            req.return_sparse,
            req.return_colbert_vecs,
        )
        cached = _cache.get(key)
        if cached is not None:
            hits[i] = cached
            cache_hits += 1
        else:
            miss_indices.append(i)
            cache_misses += 1

    # Inference for cache misses only.
    miss_results: dict = {"dense": None, "sparse": None, "colbert": None}
    if miss_indices:
        miss_texts = [texts[i] for i in miss_indices]
        miss_results = _embedder.embed(
            miss_texts,
            return_dense=req.return_dense,
            return_sparse=req.return_sparse,
            return_colbert_vecs=req.return_colbert_vecs,
            batch_size=req.batch_size,
        )
        # Store miss results in cache.
        for j, i in enumerate(miss_indices):
            d = miss_results["dense"][j] if miss_results["dense"] is not None else None
            s = miss_results["sparse"][j] if miss_results["sparse"] is not None else None
            c = miss_results["colbert"][j] if miss_results["colbert"] is not None else None
            key = _cache.key_for(
                texts[i],
                _embedder.model_name,
                _embedder.model_version,
                req.return_dense,
                req.return_sparse,
                req.return_colbert_vecs,
            )
            _cache.put(key, (d, s, c))

    # Reassemble in request order.
    dense_out: Optional[list] = [] if req.return_dense else None
    sparse_out: Optional[list] = [] if req.return_sparse else None
    colbert_out: Optional[list] = [] if req.return_colbert_vecs else None

    miss_j = 0
    for i in range(n):
        if i in hits:
            d, s, c = hits[i]
        else:
            # Pull from miss results at position miss_j.
            j = miss_indices.index(i)
            d = miss_results["dense"][j] if miss_results["dense"] is not None else None
            s = miss_results["sparse"][j] if miss_results["sparse"] is not None else None
            c = miss_results["colbert"][j] if miss_results["colbert"] is not None else None
            miss_j += 1

        if dense_out is not None:
            dense_out.append(d)
        if sparse_out is not None:
            sparse_out.append(s)
        if colbert_out is not None:
            colbert_out.append(c)

    return {
        "dense": dense_out,
        "sparse": sparse_out,
        "colbert": colbert_out,
        "cache_hits": cache_hits,
        "cache_misses": cache_misses,
    }

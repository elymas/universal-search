"""Observability helpers: JSON-structured logging + per-call timing.

REQ-IDX-002-006: Per-call JSON log record with 12 documented attributes.
"""

from __future__ import annotations

import json
import logging
import os
import sys
import time
from contextlib import contextmanager
from typing import Generator, Optional


def _setup_json_logger(name: str, level_str: str = "INFO") -> logging.Logger:
    """Return a logger that emits JSON lines to stdout."""
    level = getattr(logging, level_str.upper(), logging.INFO)
    logger = logging.getLogger(name)
    logger.setLevel(level)

    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        handler.setLevel(level)
        # Custom JSON formatter — each log record is a single-line JSON object.
        formatter = _JsonFormatter()
        handler.setFormatter(formatter)
        logger.addHandler(handler)
        logger.propagate = False

    return logger


class _JsonFormatter(logging.Formatter):
    """Formats log records as single-line JSON objects."""

    def format(self, record: logging.LogRecord) -> str:
        payload: dict = {
            "level": record.levelname,
            "event": record.getMessage(),
            "logger": record.name,
        }
        # Merge any extra kwargs passed via logger.info("msg", extra={...})
        for key, val in record.__dict__.items():
            if key not in {
                "name",
                "msg",
                "args",
                "levelname",
                "levelno",
                "pathname",
                "filename",
                "module",
                "exc_info",
                "exc_text",
                "stack_info",
                "lineno",
                "funcName",
                "created",
                "msecs",
                "relativeCreated",
                "thread",
                "threadName",
                "processName",
                "process",
                "message",
                "taskName",
            }:
                payload[key] = val
        return json.dumps(payload)


# Module-level logger — configured once per process.
_log_level = os.getenv("EMBEDDER_LOG_LEVEL", "INFO")
logger: logging.Logger = _setup_json_logger("embedder", _log_level)


@contextmanager
def timer() -> Generator[dict, None, None]:
    """Context manager that measures wall-clock elapsed time in milliseconds.

    Usage::

        with timer() as t:
            do_work()
        print(t["latency_ms"])
    """
    result: dict = {}
    start = time.perf_counter()
    yield result
    result["latency_ms"] = (time.perf_counter() - start) * 1000.0


def log_embed(
    *,
    request_id: str,
    texts_count: int,
    return_dense: bool,
    return_sparse: bool,
    return_colbert_vecs: bool,
    cache_hits: int,
    cache_misses: int,
    latency_ms: float,
    model: str,
    model_version: str,
    device: str,
    outcome: str,
) -> None:
    """Emit one INFO-level structured log record per /embed invocation.

    REQ-IDX-002-006: 12 documented attributes; outcome in allowed enum.
    Text content is NEVER logged (privacy bound per REQ acceptance criterion).
    """
    logger.info(
        "embed",
        extra={
            "request_id": request_id,
            "texts_count": texts_count,
            "return_dense": return_dense,
            "return_sparse": return_sparse,
            "return_colbert_vecs": return_colbert_vecs,
            "cache_hits": cache_hits,
            "cache_misses": cache_misses,
            "latency_ms": round(latency_ms, 3),
            "model": model,
            "model_version": model_version,
            "device": device,
            "outcome": outcome,
        },
    )

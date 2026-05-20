"""Observability helpers for the researcher service.

REQ-SYN-006: JSON-formatted stdlib logging + Timer context manager.
Per-call structured log records with 11 documented attributes.
"""

from __future__ import annotations

import json
import logging
import os
import sys
import time
from types import TracebackType
from typing import Any


class _JSONFormatter(logging.Formatter):
    """Format log records as single-line JSON objects."""

    def format(self, record: logging.LogRecord) -> str:
        data: dict[str, Any] = {
            "timestamp": self.formatTime(record),
            "level": record.levelname,
            "logger": record.name,
        }
        # Merge extra fields if they were passed as a dict in the message
        if isinstance(record.msg, dict):
            data.update(record.msg)
        else:
            data["message"] = record.getMessage()
        return json.dumps(data, ensure_ascii=False)


def setup_logging(level: str | None = None) -> None:
    """Configure the root logger for JSON output.

    Idempotent; calling multiple times does not duplicate handlers.
    """
    log_level = (level or os.getenv("RESEARCHER_LOG_LEVEL", "INFO")).upper()
    root = logging.getLogger()

    # Remove any existing handlers to avoid duplication in tests
    if root.handlers:
        root.handlers.clear()

    handler = logging.StreamHandler(sys.stderr)
    handler.setFormatter(_JSONFormatter())
    root.addHandler(handler)
    root.setLevel(getattr(logging, log_level, logging.INFO))


def log_synthesis(record: dict[str, Any]) -> None:
    """Emit a single structured JSON record at INFO level.

    REQ-SYN-006: Attributes: request_id, query_len, docs_count, model,
    provider, cost_usd, prompt_tokens, completion_tokens, latency_ms,
    degraded, outcome.
    """
    logger = logging.getLogger("researcher.synthesis")
    logger.info(record)


class Timer:
    """Context manager that measures wall-clock elapsed time in milliseconds.

    Usage::

        with Timer() as t:
            do_work()
        print(t.elapsed_ms)
    """

    def __init__(self) -> None:
        self._start: float = 0.0
        self.elapsed_ms: float = 0.0

    def __enter__(self) -> "Timer":
        self._start = time.monotonic()
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        self.elapsed_ms = (time.monotonic() - self._start) * 1000.0

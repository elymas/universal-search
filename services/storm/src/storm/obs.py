"""Observability helpers for the storm service.

JSON-formatted stdlib logging + Timer context manager + counters.
Mirrors researcher/obs.py structure.

# @MX:NOTE: [AUTO] Counter values logged via structured JSON for Prometheus scrape.
# @MX:SPEC: SPEC-DEEP-001 M3
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
        if isinstance(record.msg, dict):
            data.update(record.msg)
        else:
            data["message"] = record.getMessage()
        return json.dumps(data, ensure_ascii=False)


def setup_logging(level: str | None = None) -> None:
    """Configure the root logger for JSON output.

    Idempotent; calling multiple times does not duplicate handlers.
    """
    log_level = (level or os.getenv("STORM_LOG_LEVEL", "INFO")).upper()
    root = logging.getLogger()

    # Remove existing handlers to avoid duplication
    if root.handlers:
        root.handlers.clear()

    handler = logging.StreamHandler(sys.stderr)
    handler.setFormatter(_JSONFormatter())
    root.addHandler(handler)
    root.setLevel(getattr(logging, log_level, logging.INFO))


def log_report(record: dict[str, Any]) -> None:
    """Emit a single structured JSON record at INFO level."""
    logger = logging.getLogger("storm.report")
    logger.info(record)


class Timer:
    """Context manager that measures wall-clock elapsed time in milliseconds."""

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


class OutcomeEmitter:
    """Request-scoped outcome emitter with at-most-once guarantee.

    NFR-DEEP1-003: usearch_deep_outcomes_total counter SHALL increment
    exactly once per non-disconnect request lifecycle.

    # @MX:NOTE: [AUTO] Single-emission guard per request (sync.Once equivalent).
    """

    def __init__(self, request_id: str) -> None:
        self.request_id = request_id
        self._emitted: bool = False

    def emit(self, outcome: str, **extra: Any) -> None:
        """Emit outcome exactly once. Subsequent calls are no-ops."""
        if self._emitted:
            return
        self._emitted = True
        log_report(
            {
                "request_id": self.request_id,
                "outcome": outcome,
                **extra,
            }
        )


def emit_outcome(outcome: str, *, request_id: str, **extra: Any) -> None:
    """Module-level convenience wrapper for single-shot outcome emission.

    # @MX:NOTE: [AUTO] Thin wrapper used by app.py exception handlers.
    """
    emitter = OutcomeEmitter(request_id=request_id)
    emitter.emit(outcome, **extra)


# ---------------------------------------------------------------------------
# Citation counters (SPEC-DEEP-001 M3, REQ-DEEP1-002)
# ---------------------------------------------------------------------------

_unresolved_citations_total: int = 0


def inc_unresolved_citations(count: int = 1) -> None:
    """Increment the unresolved citations counter.

    REQ-DEEP1-002: counter ``usearch_storm_unresolved_citations_total``
    tracks citation markers that could not be matched to any input doc.
    """
    global _unresolved_citations_total
    _unresolved_citations_total += count
    log_report(
        {
            "metric": "usearch_storm_unresolved_citations_total",
            "value": _unresolved_citations_total,
        }
    )


def get_unresolved_citations_total() -> int:
    """Return the current unresolved citations counter value."""
    return _unresolved_citations_total

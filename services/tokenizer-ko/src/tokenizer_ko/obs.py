"""Observability helpers for the tokenizer-ko sidecar.

REQ-IDX-003-009: Structured JSON logging; per-call log record with
{request_id, text_len, morpheme_count, latency_ms, outcome}.
"""

from __future__ import annotations

import logging
import sys
import time

logger = logging.getLogger("tokenizer_ko")

# Valid outcome values for the log record.
OUTCOME_SUCCESS = "success"
OUTCOME_ERROR_INVALID = "error_invalid"
OUTCOME_ERROR_INTERNAL = "error_internal"


def setup_logging(level: str = "INFO") -> None:
    """Configure root logger to emit JSON-structured records on stdout.

    Uses stdlib logging with a JSON-compatible formatter. The log handler
    writes to stdout for Docker log capture.
    """
    numeric_level = getattr(logging, level.upper(), logging.INFO)
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(numeric_level)

    # Simple JSON-style formatter that outputs {key: value} pairs.
    class JSONFormatter(logging.Formatter):
        def format(self, record: logging.LogRecord) -> str:
            import json as _json

            log_dict: dict = {
                "level": record.levelname,
                "logger": record.name,
                "message": record.getMessage(),
            }
            # Attach extra fields added via record.__dict__
            for key in (
                "request_id",
                "text_len",
                "morpheme_count",
                "latency_ms",
                "outcome",
                "error",
            ):
                if hasattr(record, key):
                    log_dict[key] = getattr(record, key)

            return _json.dumps(log_dict, ensure_ascii=False)

    handler.setFormatter(JSONFormatter())
    logging.getLogger("tokenizer_ko").addHandler(handler)
    logging.getLogger("tokenizer_ko").setLevel(numeric_level)
    logging.getLogger("tokenizer_ko").propagate = False


def log_tokenize(
    *,
    request_id: str,
    text_len: int,
    morpheme_count: int,
    latency_ms: float,
    outcome: str,
) -> None:
    """Emit one INFO-level structured log record per /tokenize invocation.

    REQ-IDX-003-009(a): 5 mandatory attributes; text value NOT included (PII).
    """
    extra = {
        "request_id": request_id,
        "text_len": text_len,
        "morpheme_count": morpheme_count,
        "latency_ms": latency_ms,
        "outcome": outcome,
    }
    logger.info(
        "tokenize: request_id=%s text_len=%d morpheme_count=%d latency_ms=%.2f outcome=%s",
        request_id,
        text_len,
        morpheme_count,
        latency_ms,
        outcome,
        extra=extra,
        stacklevel=2,
    )


def log_invalid_input(*, request_id: str, error: str) -> None:
    """Emit one WARN-level log record for invalid input (REQ-IDX-003-004)."""
    extra = {"request_id": request_id, "error": error}
    logger.warning(  # nosemgrep: python.lang.security.audit.logging.logger-credential-leak -- `error` is a static validation-error string (e.g. "empty text"), never a secret
        "tokenize: invalid_input request_id=%s error=%s",
        request_id,
        error,
        extra=extra,
        stacklevel=2,
    )


class Timer:
    """Context manager that tracks elapsed time in milliseconds."""

    def __init__(self) -> None:
        self._start: float = 0.0
        self.elapsed_ms: float = 0.0

    def __enter__(self) -> "Timer":
        self._start = time.perf_counter()
        return self

    def __exit__(self, *_) -> None:
        self.elapsed_ms = (time.perf_counter() - self._start) * 1000.0

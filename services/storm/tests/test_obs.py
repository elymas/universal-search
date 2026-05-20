"""Tests for storm JSON-log observability.

RED phase: Define expected obs behavior before implementation.
"""

from __future__ import annotations

import json
import logging


class TestSetupLogging:
    """setup_logging configures JSON-formatted log output."""

    def test_setup_logging_creates_handler(self) -> None:
        """setup_logging adds a JSON handler to the root logger."""
        from storm.obs import setup_logging

        root = logging.getLogger()
        root.handlers.clear()
        setup_logging()
        assert len(root.handlers) >= 1

    def test_setup_logging_idempotent(self) -> None:
        """Calling setup_logging twice does not duplicate handlers."""
        from storm.obs import setup_logging

        root = logging.getLogger()
        root.handlers.clear()
        setup_logging()
        count_after_first = len(root.handlers)
        setup_logging()
        assert len(root.handlers) == count_after_first

    def test_setup_logging_respects_env_level(self) -> None:
        """setup_logging reads STORM_LOG_LEVEL env var."""
        import os
        from unittest.mock import patch

        from storm.obs import setup_logging

        root = logging.getLogger()
        root.handlers.clear()
        with patch.dict(os.environ, {"STORM_LOG_LEVEL": "DEBUG"}):
            setup_logging()
            assert root.level == logging.DEBUG

    def test_setup_logging_default_info(self) -> None:
        """setup_logging defaults to INFO level."""
        import os
        from unittest.mock import patch

        from storm.obs import setup_logging

        root = logging.getLogger()
        root.handlers.clear()
        with patch.dict(os.environ, {}, clear=True):
            setup_logging()
            assert root.level == logging.INFO


class TestLogReport:
    """log_report emits structured JSON records."""

    def test_log_report_emits_json(self) -> None:
        """log_report emits a valid JSON log record to stderr."""
        import io

        from storm.obs import _JSONFormatter, log_report

        buf = io.StringIO()
        handler = logging.StreamHandler(buf)
        handler.setFormatter(_JSONFormatter())
        logger = logging.getLogger("storm.report")
        logger.handlers = [handler]
        logger.setLevel(logging.INFO)
        logger.propagate = False

        log_report({"request_id": "r1", "outcome": "success"})

        output = buf.getvalue().strip()
        assert output
        parsed = json.loads(output)
        assert parsed["request_id"] == "r1"
        assert parsed["outcome"] == "success"
        logger.propagate = True

    def test_log_report_contains_fields(self) -> None:
        """log_report record contains the passed fields."""
        import io

        from storm.obs import _JSONFormatter, log_report

        buf = io.StringIO()
        handler = logging.StreamHandler(buf)
        handler.setFormatter(_JSONFormatter())
        logger = logging.getLogger("storm.report")
        logger.handlers = [handler]
        logger.setLevel(logging.INFO)
        logger.propagate = False

        log_report({"request_id": "r2", "outcome": "error_upstream"})

        output = buf.getvalue().strip()
        parsed = json.loads(output)
        assert parsed["request_id"] == "r2"
        assert parsed["outcome"] == "error_upstream"
        logger.propagate = True


class TestTimer:
    """Timer context manager measures elapsed time."""

    def test_timer_records_elapsed(self) -> None:
        """Timer records positive elapsed time."""
        import time

        from storm.obs import Timer

        with Timer() as t:
            time.sleep(0.01)
        assert t.elapsed_ms > 0

    def test_timer_zero_for_instant(self) -> None:
        """Timer records near-zero for instant execution."""
        from storm.obs import Timer

        with Timer() as t:
            pass
        assert t.elapsed_ms < 100  # Should be well under 100ms

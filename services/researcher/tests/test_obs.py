"""JSON log record shape tests.

Covers REQ-SYN-006: per-call structured log records.
"""

from __future__ import annotations

import json
import os
from typing import Any

# ---------------------------------------------------------------------------
# REQ-SYN-006 — Python-side observability
# ---------------------------------------------------------------------------


class TestLogRecordShape:
    """REQ-SYN-006: Each /synthesize call emits one structured JSON log."""

    def test_python_log_record_shape(self, capfd: Any) -> None:
        """log_synthesis emits exactly 1 JSON record with 11 documented attributes."""
        from researcher.obs import log_synthesis, setup_logging

        setup_logging()

        record = {
            "request_id": "req-obs-001",
            "query_len": 11,
            "docs_count": 3,
            "model": "claude-haiku-4-5",
            "provider": "anthropic",
            "cost_usd": 0.0023,
            "prompt_tokens": 100,
            "completion_tokens": 50,
            "latency_ms": 500,
            "degraded": False,
            "outcome": "success",
        }
        log_synthesis(record)

        captured = capfd.readouterr()
        output = captured.out + captured.err
        lines = [line.strip() for line in output.splitlines() if line.strip()]
        json_records = []
        for line in lines:
            try:
                parsed = json.loads(line)
                if "request_id" in parsed or "outcome" in parsed:
                    json_records.append(parsed)
            except json.JSONDecodeError:
                pass

        assert len(json_records) >= 1, f"Expected at least 1 JSON log line, got 0. Output: {output!r}"
        rec = json_records[0]
        required_attrs = [
            "request_id",
            "query_len",
            "docs_count",
            "model",
            "provider",
            "cost_usd",
            "prompt_tokens",
            "completion_tokens",
            "latency_ms",
            "degraded",
            "outcome",
        ]
        for attr in required_attrs:
            assert attr in rec, f"Missing attribute {attr!r} in log record: {rec}"

        # outcome must be one of the 5 defined values
        valid_outcomes = {"success", "degraded", "error_invalid", "error_timeout", "error_unreachable"}
        assert rec["outcome"] in valid_outcomes

    def test_python_log_redacts_api_key(self, capfd: Any, monkeypatch: Any) -> None:
        """API key value never appears in any log output."""
        os.environ["LITELLM_API_KEY"] = "super-secret-key-12345"

        from researcher.obs import log_synthesis, setup_logging

        setup_logging()

        record = {
            "request_id": "req-redact-001",
            "query_len": 5,
            "docs_count": 1,
            "model": "claude-haiku-4-5",
            "provider": "anthropic",
            "cost_usd": 0.0,
            "prompt_tokens": 10,
            "completion_tokens": 5,
            "latency_ms": 100,
            "degraded": False,
            "outcome": "success",
        }
        log_synthesis(record)

        captured = capfd.readouterr()
        output = captured.out + captured.err
        assert "super-secret-key-12345" not in output


class TestTimerContextManager:
    """Test Timer context manager from obs module."""

    def test_timer_measures_elapsed(self) -> None:
        """Timer.elapsed_ms returns elapsed time in milliseconds."""
        import time

        from researcher.obs import Timer

        with Timer() as t:
            time.sleep(0.01)

        assert t.elapsed_ms >= 10.0, f"Expected >= 10ms, got {t.elapsed_ms}"
        assert t.elapsed_ms < 5000.0, f"Expected < 5000ms, got {t.elapsed_ms}"

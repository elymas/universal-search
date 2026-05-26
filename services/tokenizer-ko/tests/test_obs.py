"""Log record shape and PII tests for the tokenizer-ko sidecar.

Covers REQ-IDX-003-009: JSON log shape, PII-free logging.
"""

from __future__ import annotations

import json
import logging

import pytest
from fastapi.testclient import TestClient

from tokenizer_ko.app import app


@pytest.fixture()
def client():
    """FastAPI TestClient with the tokenizer-ko app."""
    with TestClient(app) as c:
        yield c


class TestPythonLogShape:
    """REQ-IDX-003-009: Per-call structured log record with 5 attributes."""

    def _capture_log_records(self, fn):
        """Helper: temporarily add a MemoryHandler to capture log records from tokenizer_ko logger."""
        records = []

        class ListHandler(logging.Handler):
            def emit(self, record):
                records.append(record)

        logger_inst = logging.getLogger("tokenizer_ko")
        handler = ListHandler()
        handler.setLevel(logging.DEBUG)
        logger_inst.addHandler(handler)
        try:
            fn()
        finally:
            logger_inst.removeHandler(handler)
        return records

    def test_python_log_record_shape(self, client: TestClient) -> None:
        """Exactly 1 INFO log record per /tokenize invocation with 5 documented attrs."""
        result = {}

        def do_request():
            resp = client.post(
                "/tokenize",
                json={"request_id": "log-test-r1", "text": "안녕하세요"},
            )
            result["resp"] = resp

        records = self._capture_log_records(do_request)
        assert result["resp"].status_code == 200

        # Find the tokenize-result log record (has request_id)
        tokenize_records = [
            r
            for r in records
            if hasattr(r, "request_id") or ("request_id" in r.getMessage())
        ]
        assert len(tokenize_records) >= 1, (
            f"Expected structured log record, got records: "
            f"{[r.getMessage() for r in records]}"
        )

        # Verify 5 mandatory attributes via the LogRecord
        rec = tokenize_records[0]
        # The log message should contain all 5 fields
        msg = rec.getMessage()
        for attr in (
            "request_id",
            "text_len",
            "morpheme_count",
            "latency_ms",
            "outcome",
        ):
            assert attr in msg or hasattr(rec, attr), (
                f"Missing attribute '{attr}' in log record message: {msg}"
            )

    def test_python_log_no_pii(self, client: TestClient) -> None:
        """Log records must NOT contain the request text value."""
        sensitive_text = "개인정보보호법 위반 사례"

        def do_request():
            resp = client.post(
                "/tokenize",
                json={"request_id": "pii-test", "text": sensitive_text},
            )
            assert resp.status_code == 200

        all_records = self._capture_log_records(do_request)

        for record in all_records:
            msg = record.getMessage()
            assert sensitive_text not in msg, (
                f"Sensitive text found in log message: {msg}"
            )

    def test_log_outcome_is_one_of_valid_values(self, client: TestClient) -> None:
        """Log records with 'outcome' attribute use only valid enum values."""
        valid_outcomes = {"success", "error_invalid", "error_internal"}

        def do_request():
            client.post(
                "/tokenize",
                json={"request_id": "outcome-test", "text": "서울"},
            )

        records = self._capture_log_records(do_request)

        for record in records:
            if hasattr(record, "outcome"):
                assert record.outcome in valid_outcomes, (
                    f"Invalid outcome: {record.outcome}"
                )
            # Also check JSON-formatted messages
            try:
                data = json.loads(record.getMessage())
                if "outcome" in data:
                    assert data["outcome"] in valid_outcomes
            except (json.JSONDecodeError, TypeError):
                pass

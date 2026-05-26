"""Tests for POST /decompose_query endpoint.

Covers REQ-DEEP3-003 (breadth sub-queries + truncation),
REQ-DEEP3-009a (input validation + prompt context fields).
"""

from __future__ import annotations

import json
from typing import Any
from unittest import mock

from fastapi.testclient import TestClient


def _make_decompose_payload(**overrides: Any) -> dict[str, Any]:
    """Build a minimal decompose request payload."""
    base = {
        "root_query": "What are the effects of climate change?",
        "parent_query": "What are the economic effects of climate change?",
        "parent_evidence_summary": "Rising temperatures reduce crop yields by 10% per decade.",
        "breadth": 4,
    }
    base.update(overrides)
    return base


def _mock_gateway_complete(sub_queries: list[str]) -> Any:
    """Return an async mock for Gateway.complete that returns JSON sub-queries."""
    raw = json.dumps(sub_queries)
    mock_gw = mock.AsyncMock()
    mock_gw.complete.return_value = (
        raw,
        0.001,
        {"prompt_tokens": 50, "completion_tokens": 100},
        "anthropic",
        "claude-haiku-4-5",
    )
    return mock_gw


# ---------------------------------------------------------------------------
# T-C-001: POST /decompose_query returns breadth sub-queries
# REQ-DEEP3-003, REQ-DEEP3-009a
# ---------------------------------------------------------------------------


class TestDecomposeQueryReturnsBreadthSubQueries:
    """POST /decompose_query returns the requested number of sub-queries."""

    def test_decompose_query_returns_breadth_sub_queries(self) -> None:
        """When breadth=4 and LLM returns 4 sub-queries, response contains all 4."""
        from researcher.app import app
        from researcher.gateway import Gateway

        sub_queries = [
            "How does climate change affect GDP growth?",
            "What industries are most vulnerable to climate costs?",
            "What are the economic benefits of climate adaptation?",
            "How do insurance markets respond to climate risk?",
        ]

        mock_gw = _mock_gateway_complete(sub_queries)

        with (
            mock.patch.object(Gateway, "__init__", lambda self, **kw: None),
            mock.patch.object(Gateway, "complete", mock_gw.complete),
        ):
            client = TestClient(app, raise_server_exceptions=True)
            resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=4))

        assert resp.status_code == 200
        body = resp.json()
        assert "sub_queries" in body
        assert len(body["sub_queries"]) == 4
        assert body["sub_queries"] == sub_queries


# ---------------------------------------------------------------------------
# T-C-002: Truncates excess sub-queries beyond breadth
# REQ-DEEP3-003
# ---------------------------------------------------------------------------


class TestDecomposeQueryTruncatesExcess:
    """LLM returning more than breadth sub-queries gets truncated."""

    def test_decompose_query_truncates_excess(self) -> None:
        """When breadth=4 and LLM returns 6 sub-queries, response is truncated to 4."""
        from researcher.app import app
        from researcher.gateway import Gateway

        sub_queries = [
            "Query 1",
            "Query 2",
            "Query 3",
            "Query 4",
            "Query 5 (excess)",
            "Query 6 (excess)",
        ]

        mock_gw = _mock_gateway_complete(sub_queries)

        with (
            mock.patch.object(Gateway, "__init__", lambda self, **kw: None),
            mock.patch.object(Gateway, "complete", mock_gw.complete),
        ):
            client = TestClient(app, raise_server_exceptions=True)
            resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=4))

        assert resp.status_code == 200
        body = resp.json()
        assert len(body["sub_queries"]) == 4
        assert body["sub_queries"] == sub_queries[:4]


# ---------------------------------------------------------------------------
# T-C-003: Input validation
# REQ-DEEP3-009a
# ---------------------------------------------------------------------------


class TestDecomposeQueryValidatesInput:
    """Input validation returns 400 for invalid requests."""

    def test_decompose_query_rejects_zero_breadth(self) -> None:
        """breadth=0 returns HTTP 400."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=0))

        assert resp.status_code == 422  # Pydantic validation error

    def test_decompose_query_rejects_excess_breadth(self) -> None:
        """breadth=9 (above max 8) returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=9))

        assert resp.status_code == 422

    def test_decompose_query_requires_root_query(self) -> None:
        """Missing root_query returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["root_query"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422

    def test_decompose_query_requires_parent_query(self) -> None:
        """Missing parent_query returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["parent_query"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422

    def test_decompose_query_requires_parent_evidence_summary(self) -> None:
        """Missing parent_evidence_summary returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["parent_evidence_summary"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422

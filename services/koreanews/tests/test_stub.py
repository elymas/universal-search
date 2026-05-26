"""
KNC sidecar stub tests — SPEC-ADP-009 REQ-ADP9-009.

Run:
    pytest tests/
"""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from fastapi.testclient import TestClient
from main import app

client = TestClient(app)


def test_health_returns_200() -> None:
    """Health endpoint always returns 200 with stub status."""
    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "stub"


def test_search_returns_503() -> None:
    """Search endpoint returns 503 in stub mode."""
    resp = client.post("/search", json={"query": "한국 뉴스", "max_results": 10})
    assert resp.status_code == 503
    data = resp.json()
    assert "detail" in data
    assert "SPEC-ADP-009-KNC" in data["detail"]


def test_search_empty_body_returns_503() -> None:
    """Search endpoint returns 503 even with empty body."""
    resp = client.post("/search", json={})
    assert resp.status_code == 503


def test_search_missing_body_returns_503() -> None:
    """Search endpoint returns 503 with no body."""
    resp = client.post("/search")
    assert resp.status_code == 503

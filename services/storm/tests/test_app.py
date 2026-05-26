"""Tests for storm FastAPI app routes.

RED phase: Define expected HTTP behavior before implementation.
REQ-DEEP1-001: /health, /readyz, /generate_report endpoints.
"""

from __future__ import annotations

import os
from unittest.mock import patch

import pytest
from httpx import ASGITransport, AsyncClient


@pytest.fixture
def env_defaults() -> None:
    """Set default env vars for tests."""
    with patch.dict(
        os.environ,
        {
            "LITELLM_BASE_URL": "http://litellm:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_LOG_LEVEL": "WARNING",
        },
        clear=True,
    ):
        yield


@pytest.fixture
def app(env_defaults: None):
    """Create a fresh FastAPI app for testing."""
    from storm.app import create_app

    return create_app()


@pytest.fixture
def client(app):
    """Async HTTP client for the test app."""
    transport = ASGITransport(app=app)
    return AsyncClient(transport=transport, base_url="http://test")


class TestHealthEndpoint:
    """GET /health returns service status."""

    @pytest.mark.asyncio
    async def test_health_returns_ok(self, client: AsyncClient) -> None:
        """GET /health returns 200 with status ok."""
        async with client as c:
            resp = await c.get("/health")
        assert resp.status_code == 200
        body = resp.json()
        assert body["status"] == "ok"

    @pytest.mark.asyncio
    async def test_health_includes_version(self, client: AsyncClient) -> None:
        """GET /health includes version field."""
        async with client as c:
            resp = await c.get("/health")
        body = resp.json()
        assert "version" in body


class TestReadyzEndpoint:
    """GET /readyz returns readiness status with dependency checks."""

    @pytest.mark.asyncio
    async def test_readyz_returns_ready_true(self, client: AsyncClient) -> None:
        """GET /readyz returns ready: true when deps are available."""
        async with client as c:
            resp = await c.get("/readyz")
        assert resp.status_code == 200
        body = resp.json()
        assert "ready" in body
        assert "deps" in body

    @pytest.mark.asyncio
    async def test_readyz_deps_structure(self, client: AsyncClient) -> None:
        """GET /readyz deps contains litellm and storm_lib booleans."""
        async with client as c:
            resp = await c.get("/readyz")
        body = resp.json()
        assert "litellm" in body["deps"]
        assert "storm_lib" in body["deps"]
        assert isinstance(body["deps"]["litellm"], bool)
        assert isinstance(body["deps"]["storm_lib"], bool)


class TestGenerateReportEndpoint:
    """POST /generate_report returns report via pipeline."""

    @pytest.mark.asyncio
    async def test_post_generate_report_returns_200_with_structured_response(self, client: AsyncClient) -> None:
        """SPEC T-001 AT-1: POST valid request with mocked LM; assert 200;
        response matches GenerateReportResponse schema; schema_version == 1.
        """
        from storm.models import GenerateReportResponse

        mock_resp = GenerateReportResponse(
            request_id="at-req-001",
            title="stub",
            sections=[],
            citations=[],
            schema_version=1,
        )
        with patch("storm.app.run_pipeline", return_value=mock_resp):
            payload = {
                "request_id": "at-req-001",
                "query": "What is STORM?",
                "docs": [
                    {
                        "id": "doc-1",
                        "url": "https://example.com",
                        "title": "Example",
                        "body": "text",
                    },
                ],
            }
            async with client as c:
                resp = await c.post("/generate_report", json=payload)
        assert resp.status_code == 200
        body = resp.json()
        # Verify response matches GenerateReportResponse schema
        assert "request_id" in body
        assert "title" in body
        assert "sections" in body
        assert "citations" in body
        assert "schema_version" in body
        # Verify schema_version == 1
        assert body["schema_version"] == 1

    @pytest.mark.asyncio
    async def test_response_schema_version_present(self, client: AsyncClient) -> None:
        """SPEC T-001 AT-2: every successful response carries schema_version: 1."""
        from storm.models import GenerateReportResponse

        mock_resp = GenerateReportResponse(
            request_id="sv-req-001",
            title="stub",
            sections=[],
            citations=[],
            schema_version=1,
        )
        with patch("storm.app.run_pipeline", return_value=mock_resp):
            payload = {
                "request_id": "sv-req-001",
                "query": "test",
                "docs": [{"id": "d1", "url": "https://x.com", "title": "X", "body": "b"}],
            }
            async with client as c:
                resp = await c.post("/generate_report", json=payload)
        assert resp.status_code == 200
        assert resp.json()["schema_version"] == 1

    @pytest.mark.asyncio
    async def test_generate_report_stub_response(self, client: AsyncClient) -> None:
        """POST /generate_report returns report via mocked pipeline."""
        from storm.models import GenerateReportResponse

        mock_resp = GenerateReportResponse(
            request_id="req-001",
            title="stub",
            sections=[],
            citations=[],
            schema_version=1,
        )
        with patch("storm.app.run_pipeline", return_value=mock_resp):
            payload = {
                "request_id": "req-001",
                "query": "What is STORM?",
                "docs": [
                    {
                        "id": "doc-1",
                        "url": "https://example.com",
                        "title": "Example",
                        "body": "text",
                    },
                ],
            }
            async with client as c:
                resp = await c.post("/generate_report", json=payload)
        assert resp.status_code == 200
        body = resp.json()
        assert body["request_id"] == "req-001"
        assert body["title"] == "stub"
        assert body["sections"] == []
        assert body["citations"] == []
        assert body["schema_version"] == 1

    @pytest.mark.asyncio
    async def test_generate_report_preserves_request_id(self, client: AsyncClient) -> None:
        """POST /generate_report echoes back the request_id."""
        from storm.models import GenerateReportResponse

        mock_resp = GenerateReportResponse(
            request_id="unique-req-42",
            title="test",
            sections=[],
            citations=[],
            schema_version=1,
        )
        with patch("storm.app.run_pipeline", return_value=mock_resp):
            payload = {
                "request_id": "unique-req-42",
                "query": "test query",
                "docs": [],
            }
            async with client as c:
                resp = await c.post("/generate_report", json=payload)
        assert resp.json()["request_id"] == "unique-req-42"

    @pytest.mark.asyncio
    async def test_generate_report_invalid_body_returns_422(self, client: AsyncClient) -> None:
        """POST /generate_report returns 422 for invalid request body."""
        async with client as c:
            resp = await c.post("/generate_report", json={"invalid": "data"})
        assert resp.status_code == 422

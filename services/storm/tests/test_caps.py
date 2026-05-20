"""Tests for deadline and budget enforcement — SPEC-DEEP-001 M5.

RED phase: Define expected deadline/budget behaviour before implementation.
REQ-DEEP1-004: Hard ctx deadline at STORM_MAX_LATENCY_MS and cumulative
cost cap at STORM_MAX_COST_USD; clean cancellation paths; structured
error responses.

Tests:
- test_deadline_exceeded_returns_504
- test_deadline_exceeded_increments_counter
- test_deadline_exceeded_no_partial_text_in_response
- test_deadline_exceeded_emits_warn_log
- test_budget_exceeded_returns_402
- test_budget_exceeded_increments_counter
- test_per_call_override_clamped_to_ceiling
- test_per_call_override_clamped_logs_warning
- test_no_resource_leak_on_cancel
"""

from __future__ import annotations

import os
from unittest.mock import patch

import pytest
from httpx import ASGITransport, AsyncClient

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def env_defaults() -> None:
    """Set default env vars for tests."""
    with patch.dict(
        os.environ,
        {
            "LITELLM_BASE_URL": "http://litellm:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_LOG_LEVEL": "WARNING",
            "STORM_MAX_LATENCY_MS": "300000",
            "STORM_MAX_COST_USD": "2.50",
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


def _valid_payload(**overrides) -> dict:
    """Return a valid /generate_report payload."""
    base = {
        "request_id": "req-cap-001",
        "query": "deadline test query",
        "docs": [
            {
                "id": "doc-1",
                "url": "https://example.com",
                "title": "Example",
                "body": "text",
            },
        ],
    }
    base.update(overrides)
    return base


# ---------------------------------------------------------------------------
# Deadline tests
# ---------------------------------------------------------------------------


class TestDeadlineExceeded:
    """REQ-DEEP1-004: IF latency exceeds STORM_MAX_LATENCY_MS, THEN HTTP 504."""

    @pytest.mark.asyncio
    async def test_deadline_exceeded_returns_504(
        self, client: AsyncClient
    ) -> None:
        """DeadlineExceededError from pipeline produces HTTP 504."""
        from storm.pipeline import DeadlineExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=DeadlineExceededError(
                elapsed_ms=300_500,
                partial_sections_completed=2,
            ),
        ):
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 504
        body = resp.json()
        assert body["error"] == "deadline_exceeded"
        assert "detail" in body
        assert body["elapsed_ms"] == 300_500
        assert body["partial_sections_completed"] == 2

    @pytest.mark.asyncio
    async def test_deadline_exceeded_increments_counter(
        self, client: AsyncClient
    ) -> None:
        """Deadline exceeded path calls emit_outcome exactly once with correct label."""
        from storm.pipeline import DeadlineExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=DeadlineExceededError(
                elapsed_ms=300_100,
                partial_sections_completed=1,
            ),
        ), patch("storm.app.emit_outcome") as mock_emit:
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 504
        mock_emit.assert_called_once()
        # First positional arg is the outcome label
        assert mock_emit.call_args[0][0] == "deadline_exceeded"

    @pytest.mark.asyncio
    async def test_deadline_exceeded_no_partial_text_in_response(
        self, client: AsyncClient
    ) -> None:
        """504 response body MUST NOT contain title, sections, or citations."""
        from storm.pipeline import DeadlineExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=DeadlineExceededError(
                elapsed_ms=300_200,
                partial_sections_completed=3,
            ),
        ):
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 504
        body = resp.json()
        assert "title" not in body
        assert "sections" not in body
        assert "citations" not in body


# ---------------------------------------------------------------------------
# Budget tests
# ---------------------------------------------------------------------------


class TestBudgetExceeded:
    """REQ-DEEP1-004: IF cost exceeds STORM_MAX_COST_USD, THEN HTTP 402."""

    @pytest.mark.asyncio
    async def test_budget_exceeded_returns_402(
        self, client: AsyncClient
    ) -> None:
        """BudgetExceededError from pipeline produces HTTP 402."""
        from storm.pipeline import BudgetExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=BudgetExceededError(
                cost_usd=2.78,
                cap_usd=2.50,
            ),
        ):
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 402
        body = resp.json()
        assert body["error"] == "budget_exceeded"
        assert "detail" in body
        assert body["cost_usd"] == 2.78
        assert body["cap_usd"] == 2.50

    @pytest.mark.asyncio
    async def test_budget_exceeded_increments_counter(
        self, client: AsyncClient
    ) -> None:
        """Budget exceeded path calls emit_outcome exactly once with correct label."""
        from storm.pipeline import BudgetExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=BudgetExceededError(
                cost_usd=3.00,
                cap_usd=2.50,
            ),
        ), patch("storm.app.emit_outcome") as mock_emit:
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 402
        mock_emit.assert_called_once()
        # First positional arg is the outcome label
        assert mock_emit.call_args[0][0] == "budget_exceeded"


# ---------------------------------------------------------------------------
# Per-call override clamping tests
# ---------------------------------------------------------------------------


class TestPerCallOverrideClamping:
    """Per-call max_latency_ms is clamped to STORM_MAX_LATENCY_MS ceiling."""

    @pytest.mark.asyncio
    async def test_per_call_override_clamped_to_ceiling(
        self, client: AsyncClient
    ) -> None:
        """When request.max_latency_ms > STORM_MAX_LATENCY_MS, use the ceiling."""
        from storm.pipeline import compute_effective_deadline_ms

        # request asks for 600000ms but ceiling is 300000ms
        effective = compute_effective_deadline_ms(
            request_max_ms=600_000,
            ceiling_ms=300_000,
        )
        assert effective == 300_000

    @pytest.mark.asyncio
    async def test_per_call_override_clamped_logs_warning(
        self, client: AsyncClient
    ) -> None:
        """Clamping emits a WARN-level log with override details."""
        from storm.pipeline import compute_effective_deadline_ms

        with patch("storm.pipeline.logger") as mock_logger:
            compute_effective_deadline_ms(
                request_max_ms=600_000,
                ceiling_ms=300_000,
            )
            mock_logger.warning.assert_called_once()
            call_args = mock_logger.warning.call_args
            log_data = call_args[0][0] if call_args[0] else call_args[1]
            assert isinstance(log_data, dict)
            assert log_data.get("reason") == "max_latency_ms_clamped"


# ---------------------------------------------------------------------------
# Deadline warn log test
# ---------------------------------------------------------------------------


class TestDeadlineWarnLog:
    """REQ-DEEP1-004: deadline exceeded emits WARN-level structured log."""

    @pytest.mark.asyncio
    async def test_deadline_exceeded_emits_warn_log(
        self, client: AsyncClient
    ) -> None:
        """504 handler emits WARN log with correct attributes."""
        import logging

        from storm.pipeline import DeadlineExceededError

        with patch(
            "storm.app.run_pipeline",
            side_effect=DeadlineExceededError(
                elapsed_ms=301_000,
                partial_sections_completed=4,
            ),
        ), patch.object(logging.getLogger("storm.app"), "warning") as mock_warn:
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 504
        mock_warn.assert_called_once()
        call_args = mock_warn.call_args
        log_data = call_args[0][0] if call_args[0] else call_args[1]
        assert isinstance(log_data, dict)
        assert log_data.get("reason") == "deadline_exceeded"
        assert log_data.get("elapsed_ms") == 301_000
        assert log_data.get("partial_sections_completed") == 4


# ---------------------------------------------------------------------------
# Resource leak test
# ---------------------------------------------------------------------------


class TestResourceLeakOnCancel:
    """REQ-DEEP1-004a: no thread leak after cancellation."""

    @pytest.mark.asyncio
    async def test_no_resource_leak_on_cancel(self, client: AsyncClient) -> None:
        """Thread count returns to baseline after deadline cancellation path."""
        import threading

        from storm.pipeline import DeadlineExceededError

        baseline = len(threading.enumerate())

        with patch(
            "storm.app.run_pipeline",
            side_effect=DeadlineExceededError(
                elapsed_ms=300_100,
                partial_sections_completed=1,
            ),
        ):
            async with client as c:
                resp = await c.post(
                    "/generate_report", json=_valid_payload()
                )
        assert resp.status_code == 504

        after = len(threading.enumerate())
        assert after == baseline

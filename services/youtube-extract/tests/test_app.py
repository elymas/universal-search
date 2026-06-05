"""Tests for FastAPI app endpoints: /health and /search.

All tests use mocked yt-dlp subprocess (no live network, ADP-005 D8).
"""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient


# ---------------------------------------------------------------------------
# /health endpoint tests (REQ-ADP5a-002)
# ---------------------------------------------------------------------------


class TestHealthEndpoint:
    """REQ-ADP5a-002: GET /health."""

    def test_health_returns_ok_when_ready(self, client: TestClient) -> None:
        """200 + status=='ok' + ytdlp_version when ready."""
        resp = client.get("/health")
        assert resp.status_code == 200
        body = resp.json()
        assert body["status"] == "ok"
        assert "ytdlp_version" in body
        assert body["ytdlp_version"] == "2026.03.17"

    def test_health_returns_503_while_loading(self, client_not_ready: TestClient) -> None:
        """503 + non-'ok' status while not ready."""
        resp = client_not_ready.get("/health")
        assert resp.status_code == 503
        body = resp.json()
        assert body["status"] != "ok"


# ---------------------------------------------------------------------------
# /search endpoint tests (REQ-ADP5a-003, REQ-ADP5a-006)
# ---------------------------------------------------------------------------


class TestSearchEndpoint:
    """REQ-ADP5a-003: POST /search."""

    def _mock_ytdlp(self, entries: list[dict]) -> AsyncMock:
        """Create a mock subprocess that returns JSON lines for entries."""
        stdout = "\n".join(json.dumps(e) for e in entries)
        from tests.conftest import make_mock_process

        return make_mock_process(stdout=stdout, returncode=0)

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_search_happy_path_returns_items_envelope(
        self, mock_exec: AsyncMock, client: TestClient, sample_ytdlp_entries: list[dict]
    ) -> None:
        """POST /search returns {"items":[...],"has_more":bool} with correct field names."""
        mock_exec.return_value = self._mock_ytdlp(sample_ytdlp_entries)

        resp = client.post(
            "/search",
            json={
                "query": "go generics tutorial",
                "max_results": 25,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()

        # Envelope shape.
        assert "items" in body
        assert "has_more" in body
        assert isinstance(body["items"], list)

        # Field names match parse.go exactly.
        item = body["items"][0]
        expected_fields = {
            "id", "url", "title", "description", "channel", "channel_id",
            "channel_url", "uploader", "uploader_id", "duration_seconds",
            "view_count", "like_count", "upload_date", "thumbnail_url",
            "tags", "available_transcript_langs", "transcript_snippet",
            "transcript_lang", "transcript_is_auto", "error",
        }
        assert set(item.keys()) == expected_fields

        # upload_date is YYYY-MM-DD format (reformatted from YYYYMMDD).
        assert item["upload_date"] == "2009-10-25"

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_search_with_cursor_offset_slices_results(
        self, mock_exec: AsyncMock, client: TestClient, sample_ytdlp_entries: list[dict]
    ) -> None:
        """Results are sliced by cursor_offset."""
        mock_exec.return_value = self._mock_ytdlp(sample_ytdlp_entries)

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 1,
                "cursor_offset": 1,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        # Only 1 item returned (offset=1, max=1 out of 2).
        assert len(body["items"]) == 1
        assert body["items"][0]["id"] == "abcdef12345"
        # has_more should be False since only 2 items total.
        assert body["has_more"] is False

    def test_search_request_validates_required_fields(self, client: TestClient) -> None:
        """Missing query or wrong type returns 4xx + permanent envelope."""
        # Missing query entirely (Pydantic validation).
        resp = client.post("/search", json={"max_results": 10})
        assert resp.status_code == 422

        # Empty query (business validation).
        resp = client.post(
            "/search",
            json={
                "query": "",
                "max_results": 10,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 400
        body = resp.json()
        assert "error" in body
        assert body["error"]["category"] == "permanent"

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_search_returns_empty_items_when_no_results(
        self, mock_exec: AsyncMock, client: TestClient
    ) -> None:
        """Empty search results return empty items list."""
        mock_exec.return_value = self._mock_ytdlp([])

        resp = client.post(
            "/search",
            json={
                "query": "nonexistent query xyz123",
                "max_results": 25,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["items"] == []
        assert body["has_more"] is False

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_signed_in_challenge_returns_503_envelope(
        self, mock_exec: AsyncMock, client: TestClient
    ) -> None:
        """REQ-ADP5a-006: 'Sign in to confirm' → 503 + unavailable envelope."""
        from tests.conftest import make_mock_process

        mock_exec.return_value = make_mock_process(
            stdout="",
            stderr="ERROR: Sign in to confirm you're not a bot",
            returncode=1,
        )

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 5,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 503
        body = resp.json()
        assert body["error"]["category"] == "unavailable"
        assert "signed-in challenge" in body["error"]["message"]

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_rate_limit_returns_429_retry_after(
        self, mock_exec: AsyncMock, client: TestClient
    ) -> None:
        """REQ-ADP5a-006: Rate-limiting → 429 + Retry-After header."""
        from tests.conftest import make_mock_process

        mock_exec.return_value = make_mock_process(
            stdout="",
            stderr="ERROR: HTTP Error 429: Too Many Requests",
            returncode=1,
        )

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 5,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 429
        body = resp.json()
        assert body["error"]["category"] == "rate_limited"
        assert "Retry-After" in resp.headers

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_search_has_more_true_when_more_results(
        self, mock_exec: AsyncMock, client: TestClient, sample_ytdlp_entries: list[dict]
    ) -> None:
        """has_more is true when more results exist beyond the returned slice."""
        mock_exec.return_value = self._mock_ytdlp(sample_ytdlp_entries)

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 1,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["has_more"] is True
        assert len(body["items"]) == 1

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_view_count_null_for_livestream(
        self, mock_exec: AsyncMock, client: TestClient, livestream_entry: dict
    ) -> None:
        """REQ-ADP5a-004: livestream → view_count: null."""
        mock_exec.return_value = self._mock_ytdlp([livestream_entry])

        resp = client.post(
            "/search",
            json={
                "query": "live stream",
                "max_results": 10,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert len(body["items"]) == 1
        assert body["items"][0]["view_count"] is None

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_available_transcript_langs_always_array(
        self, mock_exec: AsyncMock, client: TestClient, sample_ytdlp_entries: list[dict]
    ) -> None:
        """REQ-ADP5a-004: available_transcript_langs always array, never null."""
        mock_exec.return_value = self._mock_ytdlp(sample_ytdlp_entries)

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 10,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        for item in body["items"]:
            assert isinstance(item["available_transcript_langs"], list)

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_upload_date_reformatted_yyyy_mm_dd(
        self, mock_exec: AsyncMock, client: TestClient, sample_ytdlp_entries: list[dict]
    ) -> None:
        """REQ-ADP5a-004: YYYYMMDD → YYYY-MM-DD."""
        mock_exec.return_value = self._mock_ytdlp(sample_ytdlp_entries)

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 10,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        dates = [item["upload_date"] for item in body["items"] if item["upload_date"]]
        for d in dates:
            assert len(d) == 10
            assert d[4] == "-"
            assert d[7] == "-"

    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    def test_partial_failure_item_carries_error(
        self,
        mock_exec: AsyncMock,
        client: TestClient,
        sample_ytdlp_entries: list[dict],
        partial_error_entry: dict,
    ) -> None:
        """REQ-ADP5a-004: per-item error field is non-null for failed items."""
        all_entries = sample_ytdlp_entries + [partial_error_entry]
        mock_exec.return_value = self._mock_ytdlp(all_entries)

        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 10,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        # The error item should have non-null error field.
        error_items = [i for i in body["items"] if i.get("error") is not None]
        assert len(error_items) >= 1
        assert error_items[0]["error"] == "Video unavailable"

    @patch("youtube_extract.app.run_search", side_effect=RuntimeError("boom"))
    def test_search_unexpected_error_returns_500(self, mock_run: AsyncMock, client: TestClient) -> None:
        """Unexpected exception returns 500 + unavailable envelope."""
        resp = client.post(
            "/search",
            json={
                "query": "test",
                "max_results": 5,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 500
        body = resp.json()
        assert body["error"]["category"] == "unavailable"
        assert "internal error" in body["error"]["message"]

    def test_search_returns_503_when_not_ready(self, client_not_ready: TestClient) -> None:
        """Search returns 503 when sidecar is not ready."""
        resp = client_not_ready.post(
            "/search",
            json={
                "query": "test",
                "max_results": 5,
                "cursor_offset": 0,
                "transcript_lang": "en",
                "include_transcripts": True,
            },
        )
        assert resp.status_code == 503

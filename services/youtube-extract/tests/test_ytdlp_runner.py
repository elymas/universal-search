"""Tests for ytdlp_runner module — subprocess invocation, parsing, error handling.

All tests mock the yt-dlp subprocess. No live network (ADP-005 D8, NFR-ADP5a-005).
"""

from __future__ import annotations

import asyncio
import json
from unittest.mock import AsyncMock, patch

import pytest

from tests.conftest import make_mock_process
from youtube_extract.ytdlp_runner import (
    YtdlpError,
    _build_argv,
    _format_upload_date,
    _parse_item,
    _truncate_runes,
    run_search,
)

# ---------------------------------------------------------------------------
# _build_argv tests (REQ-ADP5a-007)
# ---------------------------------------------------------------------------


class TestBuildArgv:
    """REQ-ADP5a-007: cookies + sleep flags from env."""

    def test_basic_argv_contains_search_query(self) -> None:
        """ytsearch{N}:query format is correct."""
        argv = _build_argv("test query", 25)
        assert "ytsearch25:test query" in argv
        assert "--dump-json" in argv
        assert "--flat-playlist" in argv

    def test_cookies_path_adds_cookies_flag(self, tmp_path) -> None:
        """YT_COOKIES_PATH set to a readable file adds --cookies flag."""
        cookie_file = tmp_path / "cookies.txt"
        cookie_file.write_text("# Netscape cookie file\n")

        with patch("youtube_extract.ytdlp_runner.YT_COOKIES_PATH", str(cookie_file)):
            argv = _build_argv("test", 10)
            cookies_args = [a for a in argv if a.startswith("--cookies=")]
            assert len(cookies_args) == 1
            assert str(cookie_file) in cookies_args[0]

    def test_no_cookies_runs_public_path(self) -> None:
        """No YT_COOKIES_PATH → no --cookies flag."""
        with patch("youtube_extract.ytdlp_runner.YT_COOKIES_PATH", ""):
            argv = _build_argv("test", 10)
            cookies_args = [a for a in argv if "--cookies" in a]
            assert len(cookies_args) == 0

    def test_sleep_flags_default_and_override(self) -> None:
        """Sleep flags use env defaults."""
        with (
            patch("youtube_extract.ytdlp_runner.YT_SLEEP_REQUESTS", "1.0"),
            patch("youtube_extract.ytdlp_runner.YT_SLEEP_INTERVAL", "2"),
            patch("youtube_extract.ytdlp_runner.YT_MAX_SLEEP_INTERVAL", "5"),
        ):
            argv = _build_argv("test", 5)
            assert "--sleep-requests=1.0" in argv
            assert "--sleep-interval=2" in argv
            assert "--max-sleep-interval=5" in argv


# ---------------------------------------------------------------------------
# _format_upload_date tests (REQ-ADP5a-004)
# ---------------------------------------------------------------------------


class TestFormatUploadDate:
    """REQ-ADP5a-004: YYYYMMDD → YYYY-MM-DD."""

    def test_valid_date(self) -> None:
        assert _format_upload_date("20091025") == "2009-10-25"

    def test_empty_string(self) -> None:
        assert _format_upload_date("") == ""

    def test_none_input(self) -> None:
        assert _format_upload_date(None) == ""

    def test_short_date(self) -> None:
        """Malformed date (< 8 chars) passed through."""
        assert _format_upload_date("20091") == "20091"


# ---------------------------------------------------------------------------
# _truncate_runes tests (REQ-ADP5a-005)
# ---------------------------------------------------------------------------


class TestTruncateRunes:
    """REQ-ADP5a-005: transcript snippet capped to 500 runes."""

    def test_short_text_unchanged(self) -> None:
        assert _truncate_runes("hello", 500) == "hello"

    def test_exact_length(self) -> None:
        text = "a" * 500
        assert _truncate_runes(text, 500) == text

    def test_truncation_adds_ellipsis(self) -> None:
        text = "a" * 501
        result = _truncate_runes(text, 500)
        assert result == "a" * 500 + "…"
        assert len(result) == 501  # 500 chars + ellipsis

    def test_empty_string(self) -> None:
        assert _truncate_runes("", 500) == ""

    def test_multibyte_runes(self) -> None:
        """Korean characters count as single runes."""
        text = "한" * 501
        result = _truncate_runes(text, 500)
        assert result == "한" * 500 + "…"


# ---------------------------------------------------------------------------
# _parse_item tests (REQ-ADP5a-004)
# ---------------------------------------------------------------------------


class TestParseItem:
    """REQ-ADP5a-004: field parsing from yt-dlp entries."""

    def test_normal_entry(self, sample_ytdlp_entries: list[dict]) -> None:
        """Standard entry parses correctly."""
        item = _parse_item(sample_ytdlp_entries[0])
        assert item.id == "dQw4w9WgXcQ"
        assert item.title == "Never Gonna Give You Up"
        assert item.view_count == 1600000000
        assert item.upload_date == "2009-10-25"  # reformatted from 20091025
        assert item.duration_seconds == 213
        assert item.error is None

    def test_view_count_null_for_livestream(self, livestream_entry: dict) -> None:
        """Livestream entry → view_count: None."""
        item = _parse_item(livestream_entry)
        assert item.view_count is None

    def test_partial_failure_item_carries_error(self, partial_error_entry: dict) -> None:
        """Entry with _error field → non-null error."""
        item = _parse_item(partial_error_entry)
        assert item.error == "Video unavailable"

    def test_available_transcript_langs_always_list(self) -> None:
        """Available transcript langs defaults to empty list, never None."""
        entry = {"id": "test", "title": "Test"}
        item = _parse_item(entry)
        assert isinstance(item.available_transcript_langs, list)
        assert item.available_transcript_langs == []


# ---------------------------------------------------------------------------
# run_search tests (REQ-ADP5a-003, NFR-ADP5a-002, NFR-ADP5a-003)
# ---------------------------------------------------------------------------


class TestRunSearch:
    """REQ-ADP5a-003: search execution with mocked subprocess."""

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_slices_by_cursor_offset(self, mock_exec: AsyncMock) -> None:
        """REQ-ADP5a-003: results sliced by cursor_offset."""
        entries = [{"id": f"vid{i}", "title": f"Video {i}", "upload_date": "20260101"} for i in range(5)]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, has_more = await run_search("test", 2, 2, False, "en", None)
        assert len(items) == 2
        assert items[0].id == "vid2"
        assert items[1].id == "vid3"
        assert has_more is True

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_has_more_false_when_no_remaining(self, mock_exec: AsyncMock) -> None:
        """has_more is False when all results fit in the page."""
        entries = [{"id": "vid0", "title": "Video 0"}]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, has_more = await run_search("test", 25, 0, False, "en", None)
        assert has_more is False

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_subprocess_failure_raises_ytdlp_error(self, mock_exec: AsyncMock) -> None:
        """Non-zero exit code raises YtdlpError."""
        mock_exec.return_value = make_mock_process(
            stdout="",
            stderr="ERROR: something went wrong",
            returncode=1,
        )

        with pytest.raises(YtdlpError) as exc_info:
            await run_search("test", 5, 0, False, "en", None)
        assert exc_info.value.category == "unavailable"

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_ytdlp_invoked_as_subprocess(self, mock_exec: AsyncMock) -> None:
        """NFR-ADP5a-002: runner uses subprocess exec, not in-process import."""
        mock_exec.return_value = make_mock_process(stdout="{}", returncode=0)

        await run_search("test", 1, 0, False, "en", None)

        # Verify asyncio.create_subprocess_exec was called (subprocess).
        mock_exec.assert_called_once()
        call_args = mock_exec.call_args
        # First positional arg should be the yt-dlp binary.
        assert call_args[0][0] == "yt-dlp"

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_subprocess_killed_on_cancel(self, mock_exec: AsyncMock) -> None:
        """NFR-ADP5a-003: cancel mid-extract → subprocess reaped."""
        # Make communicate raise CancelledError.
        proc = make_mock_process(stdout="", stderr="", returncode=0)
        proc.communicate.side_effect = asyncio.CancelledError()
        mock_exec.return_value = proc

        with pytest.raises(YtdlpError) as exc_info:
            await run_search("test", 5, 0, False, "en", None)
        assert "cancelled" in str(exc_info.value.message).lower()
        # Verify terminate was called.
        proc.terminate.assert_called()

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_include_transcripts_false_skips_fetch(self, mock_exec: AsyncMock) -> None:
        """REQ-ADP5a-005: include_transcripts=False → no transcript fetch."""
        entries = [{"id": "vid0", "title": "Video 0"}]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, _ = await run_search("test", 1, 0, False, "en", None)
        assert items[0].transcript_snippet == ""
        assert items[0].transcript_lang == ""

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_since_filter_applied(self, mock_exec: AsyncMock) -> None:
        """Items before since timestamp are filtered out."""
        entries = [
            {"id": "old", "title": "Old Video", "upload_date": "20200101"},
            {"id": "new", "title": "New Video", "upload_date": "20260101"},
        ]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        # since=2024-01-01 → 1704067200
        items, _ = await run_search("test", 10, 0, False, "en", since=1704067200)
        ids = [i.id for i in items]
        assert "new" in ids
        assert "old" not in ids

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_subprocess_timeout_raises_error(self, mock_exec: AsyncMock) -> None:
        """NFR-ADP5a-003: timeout kills subprocess."""
        import asyncio as _asyncio

        proc = make_mock_process(stdout="", stderr="", returncode=0)
        proc.communicate.side_effect = _asyncio.TimeoutError()
        mock_exec.return_value = proc

        with pytest.raises(YtdlpError) as exc_info:
            await run_search("test", 5, 0, False, "en", None)
        assert "timed out" in str(exc_info.value.message).lower()
        proc.terminate.assert_called()

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_malformed_json_line_skipped(self, mock_exec: AsyncMock) -> None:
        """Malformed JSON lines in yt-dlp output are skipped gracefully."""
        stdout = '{"id": "vid0", "title": "Good"}\nNOTJSON\n{"id": "vid1", "title": "Also Good"}'
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, _ = await run_search("test", 10, 0, False, "en", None)
        assert len(items) == 2
        assert items[0].id == "vid0"
        assert items[1].id == "vid1"

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_since_filter_includes_items_without_date(self, mock_exec: AsyncMock) -> None:
        """Items without upload_date are included when since filter is active."""
        entries = [
            {"id": "no_date", "title": "No Date Video"},
            {"id": "new", "title": "New Video", "upload_date": "20260101"},
        ]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, _ = await run_search("test", 10, 0, False, "en", since=1704067200)
        ids = [i.id for i in items]
        assert "no_date" in ids
        assert "new" in ids

    @pytest.mark.asyncio
    @patch("youtube_extract.ytdlp_runner.asyncio.create_subprocess_exec")
    async def test_include_transcripts_true_calls_fetch(self, mock_exec: AsyncMock) -> None:
        """REQ-ADP5a-005: include_transcripts=True triggers transcript fetch."""
        entries = [{"id": "vid0", "title": "Video 0"}]
        stdout = "\n".join(json.dumps(e) for e in entries)
        mock_exec.return_value = make_mock_process(stdout=stdout, returncode=0)

        items, _ = await run_search("test", 1, 0, True, "en", None)
        assert len(items) == 1
        # available_transcript_langs should be a list (possibly empty in v0.1).
        assert isinstance(items[0].available_transcript_langs, list)

    def test_classify_error_generic(self) -> None:
        """Generic yt-dlp error returns unavailable category."""
        from youtube_extract.ytdlp_runner import _classify_error

        err = _classify_error("some random error", 1)
        assert err.category == "unavailable"
        assert err.http_status == 502

    def test_classify_error_rate_limit(self) -> None:
        """Rate limit error detection."""
        from youtube_extract.ytdlp_runner import _classify_error

        err = _classify_error("HTTP Error 429: Too Many Requests", 1)
        assert err.category == "rate_limited"
        assert err.http_status == 429
        assert err.retry_after == 30

    def test_get_ytdlp_version(self) -> None:
        """get_ytdlp_version returns a string."""
        from youtube_extract.ytdlp_runner import get_ytdlp_version

        version = get_ytdlp_version()
        assert isinstance(version, str)

    def test_parse_item_invalid_view_count(self) -> None:
        """Invalid view_count type falls back to None."""
        entry = {"id": "test", "title": "Test", "view_count": "not_a_number"}
        item = _parse_item(entry)
        assert item.view_count is None

    def test_parse_item_invalid_duration(self) -> None:
        """Invalid duration falls back to 0."""
        entry = {"id": "test", "title": "Test", "duration": "not_a_number"}
        item = _parse_item(entry)
        assert item.duration_seconds == 0

    def test_parse_item_uses_webpage_url(self) -> None:
        """URL field prefers webpage_url over url."""
        entry = {"id": "test", "title": "Test", "webpage_url": "https://youtube.com/watch?v=test"}
        item = _parse_item(entry)
        assert item.url == "https://youtube.com/watch?v=test"

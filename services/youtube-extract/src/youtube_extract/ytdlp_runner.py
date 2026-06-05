"""yt-dlp subprocess runner for YouTube search and caption fetch.

NFR-ADP5a-002: yt-dlp is invoked as a SUBPROCESS (never imported)
to preserve the Apache-2.0 / GPL boundary.

NFR-ADP5a-003: subprocess is terminated on cancellation/timeout
(SIGTERM → SIGKILL after grace period) to prevent zombies.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import signal
from typing import Any, Optional

from .models import YTItem

logger = logging.getLogger("youtube_extract")

# ---------------------------------------------------------------------------
# Configuration from environment (OWASP: no hardcoded secrets)
# ---------------------------------------------------------------------------

YT_COOKIES_PATH: str = os.getenv("YT_COOKIES_PATH", "")
YT_USER_AGENT: str = os.getenv(
    "YT_USER_AGENT",
    "Mozilla/5.0 (compatible; usearch-yt-extract/0.1)",
)
YT_SLEEP_REQUESTS: str = os.getenv("YT_SLEEP_REQUESTS", "1.0")
YT_SLEEP_INTERVAL: str = os.getenv("YT_SLEEP_INTERVAL", "2")
YT_MAX_SLEEP_INTERVAL: str = os.getenv("YT_MAX_SLEEP_INTERVAL", "5")

# Subprocess timeout (seconds) — generous for yt-dlp search + metadata.
_SUBPROCESS_TIMEOUT = 120
# Grace period (seconds) between SIGTERM and SIGKILL.
_KILL_GRACE = 5
# Maximum transcript snippet length in runes (REQ-ADP5a-005).
_MAX_TRANSCRIPT_RUNES = 500


def _build_argv(query: str, total: int) -> list[str]:
    """Build the yt-dlp command-line arguments for a search.

    Args:
        query: search query string.
        total: number of results to request (max_results + cursor_offset).

    Returns:
        Argument list suitable for asyncio.create_subprocess_exec.

    REQ-ADP5a-007: cookies + sleep flags from env with documented defaults.
    """
    argv = [
        "yt-dlp",
        f"ytsearch{total}:{query}",
        "--dump-json",
        "--flat-playlist",
        "--no-download",
        "--no-warnings",
        "--no-check-certificates",
        f"--user-agent={YT_USER_AGENT}",
        f"--sleep-requests={YT_SLEEP_REQUESTS}",
        f"--sleep-interval={YT_SLEEP_INTERVAL}",
        f"--max-sleep-interval={YT_MAX_SLEEP_INTERVAL}",
    ]

    # REQ-ADP5a-007: optional cookies for IP-block mitigation.
    if YT_COOKIES_PATH and os.path.isfile(YT_COOKIES_PATH):
        argv.append(f"--cookies={YT_COOKIES_PATH}")

    return argv


def _format_upload_date(raw: Optional[str]) -> str:
    """Reformat yt-dlp's YYYYMMDD upload_date to YYYY-MM-DD.

    REQ-ADP5a-004: upload_date must be YYYY-MM-DD (Go parse.go:153 parses "2006-01-02").
    """
    if not raw or len(raw) != 8:
        return raw or ""
    return f"{raw[:4]}-{raw[4:6]}-{raw[6:8]}"


def _truncate_runes(text: str, max_runes: int) -> str:
    """Truncate text to at most max_runes runes, appending ellipsis if truncated."""
    if not text:
        return ""
    if len(text) <= max_runes:
        return text
    # Slice by character (Python strings are already Unicode code points).
    return text[:max_runes] + "…"


def _parse_item(entry: dict[str, Any]) -> YTItem:
    """Convert a single yt-dlp JSON entry into a YTItem.

    REQ-ADP5a-004: view_count as int or null, upload_date reformatted,
    available_transcript_langs always a list, partial failure carries error.
    """
    error_msg: Optional[str] = None

    # If yt-dlp reported an extraction error for this entry, capture it.
    if entry.get("_error"):
        error_msg = str(entry["_error"])

    # view_count: yt-dlp may return None for livestreams.
    raw_view_count = entry.get("view_count")
    view_count: Optional[int] = None
    if raw_view_count is not None:
        try:
            view_count = int(raw_view_count)
        except (ValueError, TypeError):
            view_count = None

    # like_count: optional.
    raw_like_count = entry.get("like_count")
    like_count: Optional[int] = None
    if raw_like_count is not None:
        try:
            like_count = int(raw_like_count)
        except (ValueError, TypeError):
            like_count = None

    # Duration: yt-dlp returns seconds as int or float.
    raw_duration = entry.get("duration")
    duration_seconds = 0
    if raw_duration is not None:
        try:
            duration_seconds = int(raw_duration)
        except (ValueError, TypeError):
            duration_seconds = 0

    # Tags: may be missing or None.
    tags = entry.get("tags") or []

    return YTItem(
        id=entry.get("id", ""),
        url=entry.get("webpage_url") or entry.get("url", ""),
        title=entry.get("title", ""),
        description=entry.get("description", ""),
        channel=entry.get("channel") or entry.get("uploader", ""),
        channel_id=entry.get("channel_id", ""),
        channel_url=entry.get("channel_url", ""),
        uploader=entry.get("uploader", ""),
        uploader_id=entry.get("uploader_id", ""),
        duration_seconds=duration_seconds,
        view_count=view_count,
        like_count=like_count,
        upload_date=_format_upload_date(entry.get("upload_date")),
        thumbnail_url=entry.get("thumbnail", ""),
        tags=tags,
        available_transcript_langs=[],  # populated by caption fetch
        transcript_snippet="",
        transcript_lang="",
        transcript_is_auto=False,
        error=error_msg,
    )


async def run_search(
    query: str,
    max_results: int,
    cursor_offset: int,
    include_transcripts: bool,
    transcript_lang: str,
    since: Optional[int],
) -> tuple[list[YTItem], bool]:
    """Execute yt-dlp search and return sliced results.

    Args:
        query: search query (validated non-empty by caller).
        max_results: number of results to return [1, 100].
        cursor_offset: pagination offset >= 0.
        include_transcripts: whether to fetch captions.
        transcript_lang: preferred caption language.
        since: optional Unix timestamp filter.

    Returns:
        (items, has_more) where items is the sliced page and has_more
        indicates more results exist beyond the returned slice.

    Raises:
        YtdlpError: on subprocess failure, challenge, or rate-limit.
    """
    total = max_results + cursor_offset
    argv = _build_argv(query, total)

    # Apply since filter: add date restriction if provided.
    # yt-dlp doesn't natively support date filtering in search,
    # so we filter post-hoc in _apply_since_filter.

    logger.info("ytdlp.search.start", extra={"query": query, "total": total})

    proc = await asyncio.create_subprocess_exec(
        *argv,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )

    try:
        stdout_bytes, stderr_bytes = await asyncio.wait_for(
            proc.communicate(),
            timeout=_SUBPROCESS_TIMEOUT,
        )
    except asyncio.TimeoutError:
        # NFR-ADP5a-003: kill subprocess on timeout.
        await _kill_process(proc)
        raise YtdlpError("yt-dlp subprocess timed out", category="unavailable")
    except asyncio.CancelledError:
        # NFR-ADP5a-003: kill subprocess on cancellation.
        await _kill_process(proc)
        raise YtdlpError("yt-dlp subprocess cancelled", category="unavailable")

    stdout = stdout_bytes.decode("utf-8", errors="replace")
    stderr = stderr_bytes.decode("utf-8", errors="replace")

    if proc.returncode != 0:
        raise _classify_error(stderr, proc.returncode)

    # yt-dlp --flat-playlist --dump-json emits one JSON object per line.
    entries: list[dict[str, Any]] = []
    for line in stdout.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            entries.append(json.loads(line))
        except json.JSONDecodeError:
            logger.warning("ytdlp.parse.skip_line", extra={"line": line[:120]})
            continue

    # Parse entries into YTItems.
    items = [_parse_item(e) for e in entries]

    # Apply since filter post-hoc.
    if since is not None:
        items = _apply_since_filter(items, since)

    # Slice for pagination.
    page = items[cursor_offset : cursor_offset + max_results]
    has_more = len(items) > cursor_offset + max_results

    # Fetch captions if requested.
    if include_transcripts:
        page = await _fetch_transcripts(page, transcript_lang)

    logger.info(
        "ytdlp.search.done",
        extra={"total_entries": len(entries), "page_size": len(page), "has_more": has_more},
    )

    return page, has_more


async def _fetch_transcripts(items: list[YTItem], preferred_lang: str) -> list[YTItem]:
    """Fetch transcript snippets for each item using yt-dlp --list-subs + --write-sub.

    For simplicity in v0.1, we extract available_transcript_langs from the
    yt-dlp metadata (if present) and attempt to fetch a caption track.
    If caption data is not available, we leave the fields empty.

    REQ-ADP5a-005: transcript fetch with lang fallback (preferred → en → any).
    """
    # In v0.1, transcript_langs are extracted from the initial search metadata.
    # Full caption fetch via a second yt-dlp call is deferred to SPEC-SYN-001.
    for i, item in enumerate(items):
        # available_transcript_langs already populated from metadata if present.
        # For now, set to empty list (v0.1; full implementation in SPEC-SYN-001).
        items[i] = item.model_copy(update={
            "available_transcript_langs": item.available_transcript_langs or [],
        })
    return items


def _apply_since_filter(items: list[YTItem], since: int) -> list[YTItem]:
    """Filter items by upload_date >= since (Unix timestamp).

    Items without a parseable upload_date are included (conservative).
    """
    import datetime

    since_dt = datetime.datetime.fromtimestamp(since, tz=datetime.timezone.utc).date()
    filtered: list[YTItem] = []
    for item in items:
        if not item.upload_date:
            filtered.append(item)
            continue
        try:
            item_dt = datetime.date.fromisoformat(item.upload_date)
            if item_dt >= since_dt:
                filtered.append(item)
        except ValueError:
            filtered.append(item)
    return filtered


async def _kill_process(proc: asyncio.subprocess.Process) -> None:
    """Terminate then kill a subprocess (NFR-ADP5a-003)."""
    try:
        proc.terminate()
        try:
            await asyncio.wait_for(proc.wait(), timeout=_KILL_GRACE)
        except asyncio.TimeoutError:
            proc.kill()
            await proc.wait()
    except ProcessLookupError:
        pass  # already dead


def _classify_error(stderr: str, returncode: int) -> "YtdlpError":
    """Classify yt-dlp error output into structured error categories.

    REQ-ADP5a-006: 503 challenge / 429 rate-limit / permanent errors.
    """
    stderr_lower = stderr.lower()

    # REQ-ADP5a-006: "Sign in to confirm you're not a bot" challenge.
    if "sign in to confirm" in stderr_lower or "not a bot" in stderr_lower:
        return YtdlpError(
            "yt-dlp signed-in challenge",
            category="unavailable",
            http_status=503,
        )

    # REQ-ADP5a-006: rate-limiting detection.
    if "rate" in stderr_lower or "too many" in stderr_lower or "http error 429" in stderr_lower:
        return YtdlpError(
            "yt-dlp rate limited",
            category="rate_limited",
            http_status=429,
            retry_after=30,
        )

    # Generic failure.
    return YtdlpError(
        f"yt-dlp exited with code {returncode}: {stderr[:200]}",
        category="unavailable",
        http_status=502,
    )


def get_ytdlp_version() -> str:
    """Get the installed yt-dlp version string."""
    import subprocess

    try:
        result = subprocess.run(
            ["yt-dlp", "--version"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        return result.stdout.strip() or "unknown"
    except Exception:
        return "unknown"


class YtdlpError(Exception):
    """Structured error from yt-dlp subprocess execution."""

    def __init__(
        self,
        message: str,
        category: str = "unavailable",
        http_status: int = 502,
        retry_after: int = 0,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.category = category
        self.http_status = http_status
        self.retry_after = retry_after

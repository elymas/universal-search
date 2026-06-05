"""Pydantic models for the YouTube extraction sidecar.

Field names and types are frozen to match the Go adapter's wire contract exactly.
See SPEC-ADP-005a section 6.4 for the authoritative field-by-field contract.
Source: internal/adapters/youtube/parse.go:22-63 (ytSearchResponse, ytItem).
"""

from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# Request models
# ---------------------------------------------------------------------------


class SearchRequest(BaseModel):
    """Request body for POST /search.

    Matches Go searchRequestBody (search.go:37-44).
    """

    query: str
    max_results: int = 25
    cursor_offset: int = 0
    transcript_lang: str = "en"
    include_transcripts: bool = True
    since: Optional[int] = None


# ---------------------------------------------------------------------------
# Response models
# ---------------------------------------------------------------------------


class YTItem(BaseModel):
    """Single video item in the search response.

    Matches Go ytItem (parse.go:38-63).
    """

    id: str = ""
    url: str = ""
    title: str = ""
    description: str = ""
    channel: str = ""
    channel_id: str = ""
    channel_url: str = ""
    uploader: str = ""
    uploader_id: str = ""
    duration_seconds: int = 0
    # @MX:NOTE: [AUTO] view_count is Optional[int] — null for livestream-archived (parse.go:51).
    view_count: Optional[int] = None
    like_count: Optional[int] = None
    upload_date: str = ""
    thumbnail_url: str = ""
    tags: list[str] = Field(default_factory=list)
    # @MX:NOTE: [AUTO] always an array, never null/absent (REQ-ADP5a-004).
    available_transcript_langs: list[str] = Field(default_factory=list)
    transcript_snippet: str = ""
    transcript_lang: str = ""
    transcript_is_auto: bool = False
    # non-null signals per-item extraction failure; adapter skips silently (parse.go:116).
    error: Optional[str] = None


class SearchResponse(BaseModel):
    """Response envelope for POST /search.

    Matches Go ytSearchResponse (parse.go:22-27).
    """

    items: list[YTItem] = Field(default_factory=list)
    has_more: bool = False


class ErrorDetail(BaseModel):
    """Error envelope for non-200 responses.

    Matches Go ytErrEnvelope (parse.go:29-35).
    """

    category: str
    message: Optional[str] = None
    reason: Optional[str] = None


class ErrorResponse(BaseModel):
    """Top-level error wrapper."""

    error: ErrorDetail


class HealthResponse(BaseModel):
    """Response for GET /health.

    Go adapter requires status == "ok" exactly (youtube.go:142).
    """

    status: str
    ytdlp_version: Optional[str] = None

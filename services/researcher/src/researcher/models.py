"""Pydantic v2 request/response models for the researcher service.

REQ-SYN-001: SynthesizeRequest and SynthesizeResponse shapes.
NormalizedDocPayload mirrors pkg/types.NormalizedDoc (snake_case JSON).
"""

from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, ConfigDict


class NormalizedDocPayload(BaseModel):
    """Python mirror of pkg/types.NormalizedDoc.

    15-field shape; snake_case JSON; UTC ISO-8601 datetimes.
    ConfigDict(extra='forbid') ensures unknown fields are rejected.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    id: str
    source_id: str
    url: str
    title: str
    body: str
    snippet: str
    published_at: datetime
    retrieved_at: datetime
    author: str
    score: float
    lang: str
    doc_type: str
    citations: list[str] = []
    metadata: dict[str, Any] = {}
    hash: str


class SynthesizeRequest(BaseModel):
    """Request body for POST /synthesize.

    REQ-SYN-001: Pydantic v2 model with extra=forbid, str_strip_whitespace=True.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str
    query: str
    lang: str | None = None
    docs: list[NormalizedDocPayload]


class Citation(BaseModel):
    """A single citation: numeric marker + doc reference."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    marker: int
    doc_id: str
    url: str
    title: str


class SynthesizeResponse(BaseModel):
    """Response body for POST /synthesize.

    REQ-SYN-001: Full response shape including cost/token/latency metadata.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str
    text: str
    citations: list[Citation]
    model: str
    provider: str
    cost_usd: float
    prompt_tokens: int
    completion_tokens: int
    latency_ms: float
    degraded: bool
    notice: str

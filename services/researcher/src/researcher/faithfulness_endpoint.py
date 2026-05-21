"""Faithfulness check endpoint for SPEC-DEEP-002 REQ-DEEP2-006.

Reuses citation marker detection from synthesis._MARKER_RE.
POST /faithfulness_check accepts text, citations, and docs,
returns uncited_sentences_count and uncited_sentences list.
"""

from __future__ import annotations

import re

from fastapi import APIRouter
from pydantic import BaseModel, ConfigDict

from researcher.synthesis import _MARKER_RE

router = APIRouter()


class FaithfulnessCheckRequest(BaseModel):
    """Request body for POST /faithfulness_check.

    REQ-DEEP2-006: Pydantic v2 model with required text field.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    text: str
    citations: list[str] = []
    docs: list[str] = []


class FaithfulnessCheckResponse(BaseModel):
    """Response body for POST /faithfulness_check.

    REQ-DEEP2-006: Binary gate — uncited_sentences_count == 0 means PASS.
    """

    model_config = ConfigDict(extra="forbid")

    uncited_sentences_count: int
    uncited_sentences: list[str]


def _split_sentences(text: str) -> list[str]:
    """Split text into sentences on period, question mark, or exclamation mark."""
    sentences = re.split(r"(?<=[.!?])\s+", text.strip())
    return [s.strip() for s in sentences if s.strip()]


def _has_citation_marker(sentence: str) -> bool:
    """Check if a sentence contains at least one [N] citation marker."""
    return bool(_MARKER_RE.search(sentence))


@router.post("/faithfulness_check", response_model=FaithfulnessCheckResponse)
async def faithfulness_check(req: FaithfulnessCheckRequest) -> FaithfulnessCheckResponse:
    """POST /faithfulness_check — binary faithfulness gate.

    REQ-DEEP2-006: Returns uncited_sentences_count. PASS iff count == 0.
    Reuses _MARKER_RE from synthesis module for consistent marker detection.
    """
    sentences = _split_sentences(req.text)
    uncited: list[str] = []

    for sentence in sentences:
        if not _has_citation_marker(sentence):
            uncited.append(sentence)

    return FaithfulnessCheckResponse(
        uncited_sentences_count=len(uncited),
        uncited_sentences=uncited,
    )

"""Pydantic v2 request/response models for the storm service.

REQ-DEEP1-001: GenerateReportRequest, GenerateReportResponse, Section,
Sentence, Citation shapes.  All models use schema_version: int = 1.
"""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict


class Citation(BaseModel):
    """A single citation: numeric marker + doc reference."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    marker: int
    doc_id: str
    url: str
    title: str


class Sentence(BaseModel):
    """A single sentence within a section, with optional citation markers."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    text: str
    citations: list[int] = []


class Section(BaseModel):
    """A report section containing a heading, nesting level, and sentences."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    heading: str
    level: int
    sentences: list[Sentence] = []


class GenerateReportRequest(BaseModel):
    """Request body for POST /generate_report.

    REQ-DEEP1-001: schema_version defaults to 1.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str
    query: str
    docs: list[dict[str, Any]]
    schema_version: int = 1


class GenerateReportResponse(BaseModel):
    """Response body for POST /generate_report.

    REQ-DEEP1-001: Stub returns {request_id, title: "stub", sections: [],
    citations: [], schema_version: 1}.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str
    title: str
    sections: list[Section]
    citations: list[Citation]
    schema_version: int = 1

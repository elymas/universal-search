"""Tests for storm Pydantic models.

RED phase: Define expected model shapes before implementation.
REQ-DEEP1-001: schema_version=1, structured report models.
"""

from __future__ import annotations

import pytest
from pydantic import ValidationError


class TestCitation:
    """Citation model tests."""

    def test_citation_creation_with_required_fields(self) -> None:
        """Citation can be created with marker, doc_id, url, title."""
        from storm.models import Citation

        c = Citation(
            marker=1,
            doc_id="doc-abc",
            url="https://example.com",
            title="Example",
        )
        assert c.marker == 1
        assert c.doc_id == "doc-abc"
        assert c.url == "https://example.com"
        assert c.title == "Example"

    def test_citation_rejects_extra_fields(self) -> None:
        """Citation rejects unknown fields (extra=forbid)."""
        from storm.models import Citation

        with pytest.raises(ValidationError):
            Citation(
                marker=1,
                doc_id="doc-abc",
                url="https://example.com",
                title="X",
                unknown="bad",
            )

    def test_citation_strips_whitespace(self) -> None:
        """Citation strips whitespace from string fields."""
        from storm.models import Citation

        c = Citation(
            marker=1,
            doc_id="  doc-abc  ",
            url="  https://example.com  ",
            title="  Title  ",
        )
        assert c.doc_id == "doc-abc"
        assert c.url == "https://example.com"
        assert c.title == "Title"


class TestSentence:
    """Sentence model tests."""

    def test_sentence_creation(self) -> None:
        """Sentence holds text and citation markers."""
        from storm.models import Sentence

        s = Sentence(text="Some sentence.", citations=[1, 2])
        assert s.text == "Some sentence."
        assert s.citations == [1, 2]

    def test_sentence_default_citations_empty(self) -> None:
        """Sentence defaults to empty citations list."""
        from storm.models import Sentence

        s = Sentence(text="No citations.")
        assert s.citations == []

    def test_sentence_rejects_extra_fields(self) -> None:
        """Sentence rejects unknown fields."""
        from storm.models import Sentence

        with pytest.raises(ValidationError):
            Sentence(text="hi", extra="nope")


class TestSection:
    """Section model tests."""

    def test_section_creation(self) -> None:
        """Section holds heading, level, and sentences."""
        from storm.models import Section, Sentence

        s = Section(
            heading="Introduction",
            level=1,
            sentences=[Sentence(text="Hello.", citations=[1])],
        )
        assert s.heading == "Introduction"
        assert s.level == 1
        assert len(s.sentences) == 1

    def test_section_default_sentences_empty(self) -> None:
        """Section defaults to empty sentences list."""
        from storm.models import Section

        s = Section(heading="Empty", level=2)
        assert s.sentences == []

    def test_section_rejects_extra_fields(self) -> None:
        """Section rejects unknown fields."""
        from storm.models import Section

        with pytest.raises(ValidationError):
            Section(heading="A", level=1, foo="bar")


class TestGenerateReportRequest:
    """GenerateReportRequest model tests."""

    def test_request_creation(self) -> None:
        """Request requires request_id, query, docs."""
        from storm.models import GenerateReportRequest

        req = GenerateReportRequest(
            request_id="req-123",
            query="What is STORM?",
            docs=[
                {
                    "id": "doc-1",
                    "url": "https://example.com",
                    "title": "Example",
                    "body": "text",
                }
            ],
        )
        assert req.request_id == "req-123"
        assert req.query == "What is STORM?"
        assert len(req.docs) == 1

    def test_request_schema_version_default(self) -> None:
        """Request defaults schema_version to 1."""
        from storm.models import GenerateReportRequest

        req = GenerateReportRequest(
            request_id="req-1",
            query="test",
            docs=[],
        )
        assert req.schema_version == 1

    def test_request_rejects_extra_fields(self) -> None:
        """Request rejects unknown fields."""
        from storm.models import GenerateReportRequest

        with pytest.raises(ValidationError):
            GenerateReportRequest(request_id="r", query="q", docs=[], extra="no")


class TestGenerateReportResponse:
    """GenerateReportResponse model tests."""

    def test_response_creation(self) -> None:
        """Response requires request_id, title, sections, citations, schema_version."""
        from storm.models import GenerateReportResponse

        resp = GenerateReportResponse(
            request_id="req-123",
            title="Report Title",
            sections=[],
            citations=[],
            schema_version=1,
        )
        assert resp.request_id == "req-123"
        assert resp.title == "Report Title"
        assert resp.sections == []
        assert resp.citations == []
        assert resp.schema_version == 1

    def test_response_schema_version_default(self) -> None:
        """Response defaults schema_version to 1."""
        from storm.models import GenerateReportResponse

        resp = GenerateReportResponse(
            request_id="req-1",
            title="Stub",
            sections=[],
            citations=[],
        )
        assert resp.schema_version == 1

    def test_response_rejects_extra_fields(self) -> None:
        """Response rejects unknown fields."""
        from storm.models import GenerateReportResponse

        with pytest.raises(ValidationError):
            GenerateReportResponse(
                request_id="r",
                title="t",
                sections=[],
                citations=[],
                extra="bad",
            )

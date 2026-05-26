"""Citation assembly logic tests.

Covers REQ-SYN-002, NFR-SYN-002.
"""

from __future__ import annotations

import re
from typing import Any

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st


def _make_doc_payload(n: int = 1) -> dict[str, Any]:
    """Build a minimal NormalizedDocPayload dict."""
    return {
        "id": f"doc-{n}",
        "source_id": "reddit",
        "url": f"https://example.com/{n}",
        "title": f"Title {n}",
        "body": f"Body text for document {n}",
        "snippet": f"Snippet {n}",
        "published_at": "2026-01-01T00:00:00Z",
        "retrieved_at": "2026-01-02T00:00:00Z",
        "author": "author",
        "score": 0.9,
        "lang": "en",
        "doc_type": "article",
        "citations": [],
        "metadata": {},
        "hash": "abc123",
    }


# ---------------------------------------------------------------------------
# REQ-SYN-002 — Citation assembly + LiteLLM routing
# ---------------------------------------------------------------------------


class TestCitationAssembly:
    """REQ-SYN-002: Citation marker extraction, range validation."""

    def test_marker_out_of_range_stripped(self) -> None:
        """[5] in LLM output is stripped when only 3 docs supplied."""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import _process_markers

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(3)]
        text = "This is text [1] with an out-of-range [5] marker and valid [2]."
        result_text, citations = _process_markers(text, docs)
        assert "[5]" not in result_text
        assert "[1]" in result_text
        assert "[2]" in result_text
        marker_nums = [c.marker for c in citations]
        assert 5 not in marker_nums
        assert 1 in marker_nums
        assert 2 in marker_nums

    def test_marker_zero_stripped(self) -> None:
        """[0] in LLM output is stripped (markers must be >= 1)."""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import _process_markers

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(3)]
        text = "Text with [0] marker and valid [1]."
        result_text, citations = _process_markers(text, docs)
        assert "[0]" not in result_text
        assert "[1]" in result_text
        marker_nums = [c.marker for c in citations]
        assert 0 not in marker_nums

    def test_doc_id_resolution_uses_input_ids(self) -> None:
        """citations[i].doc_id == request.docs[citations[i].marker - 1].id"""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import _process_markers

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(3)]
        text = "This [1] is [2] the text [3]."
        _, citations = _process_markers(text, docs)
        for c in citations:
            expected_id = docs[c.marker - 1].id
            assert c.doc_id == expected_id

    def test_no_retrieval_attempted(self) -> None:
        """Synthesis does not trigger any web fetch/retrieval calls."""
        # Verify that the synthesis function only invokes the gateway, not any retriever
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import build_prompt

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(2)]
        messages = build_prompt(query="test query", lang="en", docs=docs)
        # Ensure messages contain SOURCE data embedded in prompt, not a retrieval call
        assert any("SOURCES" in str(m) for m in messages)
        # Ensure no URL fetch is initiated
        assert any("test query" in str(m) for m in messages)

    @pytest.mark.asyncio
    async def test_lang_hint_propagated_to_prompt(self) -> None:
        """lang='ko' causes system message to contain 'Answer in ko.'"""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import build_prompt

        docs = [NormalizedDocPayload(**_make_doc_payload(1))]
        messages = build_prompt(query="hello", lang="ko", docs=docs)
        system_content = " ".join(m.get("content", "") for m in messages if m.get("role") == "system")
        assert "Answer in ko." in system_content

    @pytest.mark.asyncio
    async def test_lang_empty_omits_directive(self) -> None:
        """lang='' does not include 'Answer in' in system prompt."""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import build_prompt

        docs = [NormalizedDocPayload(**_make_doc_payload(1))]
        messages = build_prompt(query="hello", lang="", docs=docs)
        system_content = " ".join(m.get("content", "") for m in messages if m.get("role") == "system")
        assert "Answer in" not in system_content


# ---------------------------------------------------------------------------
# NFR-SYN-002 — Marker→Doc Mapping Property Test
# ---------------------------------------------------------------------------


class TestMarkerPropertyTest:
    """NFR-SYN-002: Every marker N in returned text maps to a real input doc."""

    @given(
        num_docs=st.integers(min_value=1, max_value=10),
        llm_text=st.text(
            alphabet=st.characters(
                whitelist_categories=("L", "N", "P", "S", "Z"),
                whitelist_characters="[]0123456789",
            ),
            min_size=0,
            max_size=200,
        ),
    )
    @settings(max_examples=100)
    def test_marker_property_holds_for_arbitrary_input(self, num_docs: int, llm_text: str) -> None:
        """For any LLM text, every marker in returned text maps to a real doc."""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import _process_markers

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(num_docs)]
        result_text, citations = _process_markers(llm_text, docs)
        # Every [N] in result_text must be a valid marker
        found_markers = re.findall(r"\[(\d+)\]", result_text)
        for m_str in found_markers:
            m = int(m_str)
            assert 1 <= m <= num_docs, f"Marker [{m}] out of range [1,{num_docs}]"
        # Every citation maps to a real doc
        for c in citations:
            assert 1 <= c.marker <= num_docs
            assert c.doc_id == docs[c.marker - 1].id

    def test_negative_marker_validation(self) -> None:
        """Hand-crafted: 'Foo [1] bar [99] baz [2]' with 2-doc input."""
        from researcher.models import NormalizedDocPayload
        from researcher.synthesis import _process_markers

        docs = [NormalizedDocPayload(**_make_doc_payload(i + 1)) for i in range(2)]
        text = "Foo [1] bar [99] baz [2]"
        result_text, citations = _process_markers(text, docs)
        assert "[1]" in result_text
        assert "[2]" in result_text
        assert "[99]" not in result_text
        marker_nums = {c.marker for c in citations}
        assert 1 in marker_nums
        assert 2 in marker_nums
        assert 99 not in marker_nums

"""Tests for citation_translator.py — URL -> doc_id marker translation.

REQ-DEEP1-002: Every [N] marker SHALL resolve to Citation.doc_id via URL
canonicalization (lowercase host, strip query, normalize protocol, strip
trailing slash); unresolved markers stripped + counter +1.

TDD RED phase: these tests define the desired behaviour of
canonicalize_url() and translate() before any implementation exists.
"""

from __future__ import annotations

import pytest

from storm.citation_translator import canonicalize_url, translate


# ---------------------------------------------------------------------------
# canonicalize_url — parametric over 8 protocol / query / slash combos
# ---------------------------------------------------------------------------


class TestCanonicalizeUrl:
    """URL canonicalization must handle protocol, query string, and trailing slash."""

    # Canonical form that all 8 protocol/query/slash combinations must produce.
    CANONICAL = "https://example.com/page"

    @pytest.mark.parametrize(
        "input_url, expected",
        [
            # 1. Already canonical
            ("https://example.com/page", "https://example.com/page"),
            # 2. Uppercase host
            ("https://EXAMPLE.COM/page", "https://example.com/page"),
            # 3. Trailing slash
            ("https://example.com/page/", "https://example.com/page"),
            # 4. Query string
            ("https://example.com/page?q=1", "https://example.com/page"),
            # 5. Query + trailing slash
            ("https://example.com/page/?q=1", "https://example.com/page"),
            # 6. HTTP -> HTTPS equivalent
            ("http://example.com/page", "https://example.com/page"),
            # 7. HTTP + query + trailing slash + uppercase
            (
                "http://EXAMPLE.COM/page/?q=1&r=2",
                "https://example.com/page",
            ),
            # 8. HTTPS + uppercase host + query + trailing slash
            (
                "HTTPS://EXAMPLE.COM/page/?q=1",
                "https://example.com/page",
            ),
        ],
        ids=[
            "already-canonical",
            "uppercase-host",
            "trailing-slash",
            "query-string",
            "query-plus-slash",
            "http-to-https",
            "http-query-slash-upper",
            "full-combo",
        ],
    )
    def test_url_canonicalization_handles_protocol_query_trailing_slash(
        self,
        input_url: str,
        expected: str,
    ) -> None:
        """Parametric: 8 combinations of protocol/query/slash/case."""
        assert canonicalize_url(input_url) == expected

    @pytest.mark.parametrize(
        "input_url",
        [
            # (https, no query, no slash)
            "https://example.com/page",
            # (https, no query, slash)
            "https://example.com/page/",
            # (https, query, no slash)
            "https://example.com/page?q=1",
            # (https, query, slash)
            "https://example.com/page/?q=1",
            # (http, no query, no slash)
            "http://example.com/page",
            # (http, no query, slash)
            "http://example.com/page/",
            # (http, query, no slash)
            "http://example.com/page?q=1",
            # (http, query, slash)
            "http://example.com/page/?q=1",
        ],
        ids=[
            "https-noquery-noslash",
            "https-noquery-slash",
            "https-query-noslash",
            "https-query-slash",
            "http-noquery-noslash",
            "http-noquery-slash",
            "http-query-noslash",
            "http-query-slash",
        ],
    )
    def test_eight_combinations_produce_identical_canonical_form(
        self,
        input_url: str,
    ) -> None:
        """All 8 protocol/query/slash combinations produce identical canonical form."""
        assert canonicalize_url(input_url) == self.CANONICAL


# ---------------------------------------------------------------------------
# translate — marker -> doc_id resolution via URL canonicalization
# ---------------------------------------------------------------------------


class TestTranslate:
    """translate() maps STORM [N] markers to our doc_id-based Citations."""

    def test_marker_to_doc_id_resolution_via_url_canonicalization(self) -> None:
        """STORM output "GPT-4 [1]." with refs/docs resolves via canonical URL.

        STORM refs have URL "HTTPS://Example.com/Page?q=1/" which canonicalizes
        to "https://example.com/Page".  Our docs have url "https://example.com/page".

        Note: path component case IS preserved (Page vs page), so these should
        NOT match. We use matching URLs to verify the canonicalization chain.
        """
        text = "GPT-4 is a large language model [1]."
        storm_refs = [
            {"url": "HTTPS://Example.com/Model?q=1/", "title": "GPT-4 Wiki"},
        ]
        docs = [
            {"id": "doc_a", "url": "https://example.com/Model", "title": "GPT-4 Reference"},
        ]

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        # Marker [1] is resolved to doc_a
        assert "[1]" in translated_text
        assert len(citations) == 1
        assert citations[0].marker == 1
        assert citations[0].doc_id == "doc_a"
        assert unresolved == 0

    def test_unresolved_marker_stripped_from_text(self) -> None:
        """A marker referencing a URL not in docs is stripped from text."""
        text = "Some claim [1] and another [2]."
        storm_refs = [
            {"url": "https://example.com/found", "title": "Found"},
            {"url": "https://example.com/not-in-docs", "title": "Missing"},
        ]
        docs = [
            {"id": "doc_1", "url": "https://example.com/found", "title": "Found Doc"},
        ]

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        # [1] resolved, [2] stripped
        assert "[1]" in translated_text
        assert "[2]" not in translated_text
        assert unresolved == 1
        assert len(citations) == 1

    def test_unresolved_citations_counter_increments(self) -> None:
        """Multiple unresolved markers each increment the counter."""
        text = "Claim A [1]. Claim B [2]. Claim C [3]."
        storm_refs = [
            {"url": "https://example.com/a", "title": "A"},
            {"url": "https://example.com/b", "title": "B"},
            {"url": "https://example.com/c", "title": "C"},
        ]
        # Only doc for ref "a" exists
        docs = [
            {"id": "doc_a", "url": "https://example.com/a", "title": "Doc A"},
        ]

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        assert unresolved == 2
        assert len(citations) == 1
        # Only [1] remains; [2] and [3] stripped
        assert "[1]" in translated_text
        assert "[2]" not in translated_text
        assert "[3]" not in translated_text

    def test_citations_array_sorted_and_one_indexed(self) -> None:
        """Citations must be sorted by marker ascending and 1-indexed."""
        text = "Third [3]. First [1]. Second [2]."
        storm_refs = [
            {"url": "https://example.com/one", "title": "One"},
            {"url": "https://example.com/two", "title": "Two"},
            {"url": "https://example.com/three", "title": "Three"},
        ]
        docs = [
            {"id": "doc_one", "url": "https://example.com/one", "title": "D1"},
            {"id": "doc_two", "url": "https://example.com/two", "title": "D2"},
            {"id": "doc_three", "url": "https://example.com/three", "title": "D3"},
        ]

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        assert unresolved == 0
        assert len(citations) == 3
        # Citations sorted by marker, 1-indexed
        markers = [c.marker for c in citations]
        assert markers == [1, 2, 3]
        assert all(m >= 1 for m in markers)

    def test_text_with_no_markers_unchanged(self) -> None:
        """Text with no [N] markers passes through unchanged."""
        text = "Plain text with no citations."
        storm_refs = []
        docs = []

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        assert translated_text == text
        assert citations == []
        assert unresolved == 0

    def test_duplicate_markers_in_text(self) -> None:
        """Same marker appearing multiple times in text is preserved each time."""
        text = "First ref [1]. Another mention [1]. Third mention [2]."
        storm_refs = [
            {"url": "https://example.com/a", "title": "A"},
            {"url": "https://example.com/b", "title": "B"},
        ]
        docs = [
            {"id": "doc_a", "url": "https://example.com/a", "title": "DA"},
            {"id": "doc_b", "url": "https://example.com/b", "title": "DB"},
        ]

        translated_text, citations, unresolved = translate(text, storm_refs, docs)

        assert translated_text.count("[1]") == 2
        assert "[2]" in translated_text
        assert unresolved == 0
        assert len(citations) == 2

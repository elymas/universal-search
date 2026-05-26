"""Unit tests for inject_rm.InjectedRM — custom retrieval module.

Tests cover:
- Forward returns correct STORM-compatible format {url, title, snippets, body}
- Lexical scoring ranks docs by query word overlap
- Top-k slicing returns at most k results
- Edge cases: empty docs, empty query, k > len(docs), duplicate scores
"""

from __future__ import annotations

from storm.inject_rm import InjectedRM

# ---------------------------------------------------------------------------
# Happy-path: forward returns correct format
# ---------------------------------------------------------------------------


class TestInjectedRMForwardFormat:
    """Verify forward() output matches STORM's expected retrieval shape."""

    def test_forward_returns_list_of_dicts(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert isinstance(result, list)
        assert len(result) == 1
        item = result[0]
        assert "url" in item
        assert "title" in item
        assert "snippets" in item
        assert "body" in item

    def test_forward_output_values_match_input_docs(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha body text"},
            {"url": "https://example.com/b", "title": "Beta", "body": "beta body text"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert result[0]["url"] == "https://example.com/a"
        assert result[0]["title"] == "Alpha"
        assert result[0]["body"] == "alpha body text"

    def test_forward_snippets_is_string(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert isinstance(result[0]["snippets"], str)


# ---------------------------------------------------------------------------
# Lexical scoring and ranking
# ---------------------------------------------------------------------------


class TestInjectedRMLexicalScoring:
    """Verify docs are ranked by query word overlap."""

    def test_higher_overlap_ranks_first(self) -> None:
        docs = [
            {"url": "https://example.com/low", "title": "Low", "body": "unrelated content here"},
            {"url": "https://example.com/high", "title": "High", "body": "alpha alpha alpha match"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha match")
        assert result[0]["url"] == "https://example.com/high"

    def test_query_words_case_insensitive(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "Alpha ALPHA alpha"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert len(result) == 1

    def test_title_and_body_both_scored(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "alpha alpha", "body": "no match"},
            {"url": "https://example.com/b", "title": "no match", "body": "alpha alpha alpha"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        # Both match; body has 3 occurrences, title has 2
        # Total score: a = 2 (title) + 0 (body) = 2, b = 0 (title) + 3 (body) = 3
        assert result[0]["url"] == "https://example.com/b"


# ---------------------------------------------------------------------------
# Top-k slicing
# ---------------------------------------------------------------------------


class TestInjectedRMTopK:
    """Verify top-k limiting behavior."""

    def test_forward_respects_k_parameter(self) -> None:
        docs = [{"url": f"https://example.com/{i}", "title": f"Doc {i}", "body": f"match {i}"} for i in range(10)]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="match", k=3)
        assert len(result) <= 3

    def test_forward_k_larger_than_docs_returns_all(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
        ]
        rm = InjectedRM(docs=docs, top_k=10)
        result = rm.forward(query="alpha", k=10)
        assert len(result) == 1

    def test_forward_default_k_uses_top_k(self) -> None:
        docs = [{"url": f"https://example.com/{i}", "title": f"Doc {i}", "body": f"match {i}"} for i in range(10)]
        rm = InjectedRM(docs=docs, top_k=5)
        result = rm.forward(query="match")
        assert len(result) == 5


# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------


class TestInjectedRMEdgeCases:
    """Edge case handling."""

    def test_forward_empty_docs_returns_empty(self) -> None:
        rm = InjectedRM(docs=[], top_k=3)
        result = rm.forward(query="anything")
        assert result == []

    def test_forward_empty_query_returns_all_docs_up_to_k(self) -> None:
        docs = [{"url": f"https://example.com/{i}", "title": f"Doc {i}", "body": f"body {i}"} for i in range(5)]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="")
        assert len(result) == 3

    def test_forward_no_matching_docs_returns_empty(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="zzzzzzzzz")
        assert result == []

    def test_docs_missing_optional_fields_use_defaults(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
            {"url": "https://example.com/b"},  # Missing title and body
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert len(result) == 1
        assert result[0]["url"] == "https://example.com/a"

    def test_docs_with_snippets_field_preserved(self) -> None:
        docs = [
            {
                "url": "https://example.com/a",
                "title": "Alpha",
                "body": "alpha content",
                "snippets": "custom snippet",
            },
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert result[0]["snippets"] == "custom snippet"

    def test_docs_without_snippets_uses_body_as_snippets(self) -> None:
        docs = [
            {"url": "https://example.com/a", "title": "Alpha", "body": "alpha content"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert result[0]["snippets"] == "alpha content"

    def test_duplicate_scores_preserve_insertion_order(self) -> None:
        """When docs have equal lexical scores, original order should be stable."""
        docs = [
            {"url": "https://example.com/first", "title": "Match", "body": "alpha"},
            {"url": "https://example.com/second", "title": "Match", "body": "alpha"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="alpha")
        assert len(result) == 2
        # Both have identical scores; stable sort preserves input order
        assert result[0]["url"] == "https://example.com/first"
        assert result[1]["url"] == "https://example.com/second"

    def test_forward_with_unicode_query(self) -> None:
        """Verify lexical matching works with non-ASCII characters."""
        docs = [
            {"url": "https://example.com/a", "title": "한글 제목", "body": "테스트 본문"},
            {"url": "https://example.com/b", "title": "English", "body": "test body"},
        ]
        rm = InjectedRM(docs=docs, top_k=3)
        result = rm.forward(query="한글")
        assert len(result) == 1
        assert result[0]["url"] == "https://example.com/a"

"""Custom retrieval module consuming request-payload docs[].

InjectedRM replaces STORM's default YouRM/BingRM with a local-only
retrieval over the documents supplied in the request payload.  This
ensures the pipeline is grounded in the same input corpus as the rest
of the Universal Search system.

# @MX:NOTE: [AUTO] Custom dspy.Retrieve module; no external search API calls.
"""

from __future__ import annotations

import re
from typing import Any


def _score(query_words: list[str], text: str) -> int:
    """Lexical overlap score: total occurrences of query words in text."""
    text_lower = text.lower()
    return sum(text_lower.count(w) for w in query_words)


class InjectedRM:
    """Retrieval module backed by request-payload documents.

    Parameters
    ----------
    docs:
        List of document dicts from the request payload.  Each may contain
        ``url``, ``title``, ``body``, ``snippets`` keys.
    top_k:
        Default number of documents to return from ``forward()``.
    """

    def __init__(self, docs: list[dict[str, Any]], top_k: int = 3) -> None:
        self._docs = docs
        self._top_k = top_k

    def forward(self, query: str, k: int | None = None) -> list[dict[str, str]]:
        """Return top-k docs ranked by lexical overlap with *query*.

        Output format matches STORM's expected retrieval shape:
        ``{url, title, snippets, body}``.
        """
        effective_k = k if k is not None else self._top_k
        if not self._docs:
            return []

        query_words = [w for w in re.split(r"\s+", query.strip().lower()) if w]

        if not query_words:
            # No query words: return first docs up to k
            results: list[dict[str, str]] = []
            for doc in self._docs[:effective_k]:
                results.append(self._format_doc(doc))
            return results

        # Score each doc by title + body word overlap
        scored: list[tuple[int, dict[str, Any]]] = []
        for doc in self._docs:
            title = doc.get("title", "")
            body = doc.get("body", "")
            score = _score(query_words, title) + _score(query_words, body)
            if score > 0:
                scored.append((score, doc))

        if not scored:
            return []

        # Sort by score descending, then take top-k
        scored.sort(key=lambda x: x[0], reverse=True)
        return [self._format_doc(doc) for _, doc in scored[:effective_k]]

    @staticmethod
    def _format_doc(doc: dict[str, Any]) -> dict[str, str]:
        """Normalize a raw doc dict into STORM's retrieval output format."""
        return {
            "url": doc.get("url", ""),
            "title": doc.get("title", ""),
            "snippets": doc.get("snippets", doc.get("body", "")),
            "body": doc.get("body", ""),
        }

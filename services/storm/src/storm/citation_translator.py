"""Citation translator: STORM URL-cited [N] markers -> our doc_id-cited markers.

REQ-DEEP1-002: Every [N] marker SHALL resolve to Citation.doc_id via URL
canonicalization (lowercase host, strip query, normalize protocol, strip
trailing slash).  Unresolved markers are stripped from text and increment
the unresolved counter.

# @MX:ANCHOR: [AUTO] URL -> doc_id citation translation chokepoint.
# @MX:REASON: STORM URL-cited markers become our internal doc_id citations.
# @MX:SPEC: SPEC-DEEP-001 M3, REQ-DEEP1-002
"""

from __future__ import annotations

import re
from urllib.parse import urlparse

from storm.models import Citation

# Pre-compiled pattern for [N] citation markers (1+ digits).
_MARKER_RE = re.compile(r"\[(\d+)\]")


def canonicalize_url(url: str) -> str:
    """Canonicalize a URL for matching purposes.

    Rules:
    - Lowercase the host
    - Strip the query string
    - Strip the trailing slash from the path
    - Treat http and https as equivalent (normalize to https)
    """
    parsed = urlparse(url)
    scheme = "https"
    host = parsed.hostname.lower() if parsed.hostname else ""
    path = parsed.path
    # Strip trailing slash (but keep root "/")
    if path.endswith("/") and len(path) > 1:
        path = path.rstrip("/")
    return f"{scheme}://{host}{path}"


# @MX:TODO: [AUTO] Wire inc_unresolved_citations() into pipeline.py after M3 GREEN.

def translate(
    text: str,
    storm_refs: list[dict],
    docs: list[dict],
) -> tuple[str, list[Citation], int]:
    """Translate STORM URL-cited [N] markers to doc_id-cited markers.

    Algorithm:
    1. Build canonical-URL -> doc lookup from docs[]
    2. For each [N] found in text, look up storm_refs[N-1].url
    3. Canonicalize and match against docs
    4. On match: keep marker (renumbered to 1-indexed position)
    5. On no-match: strip marker, increment unresolved counter

    Returns (translated_text, citations_list, unresolved_count).
    """
    # Build lookup: canonical_url -> doc dict
    doc_lookup: dict[str, dict] = {}
    for doc in docs:
        c_url = canonicalize_url(doc.get("url", ""))
        doc_lookup[c_url] = doc

    # Build ref index: storm_ref number (1-based) -> canonical url
    ref_urls: dict[int, str] = {}
    for idx, ref in enumerate(storm_refs, start=1):
        num = ref.get("n", ref.get("number", idx))
        ref_urls[num] = canonicalize_url(ref.get("url", ""))

    # Single pass: classify each unique marker as resolved or unresolved.
    resolved: dict[int, dict] = {}  # original_marker -> doc
    unresolved_markers: set[int] = set()

    for match in _MARKER_RE.finditer(text):
        marker_num = int(match.group(1))
        if marker_num in resolved or marker_num in unresolved_markers:
            continue  # Already classified

        ref_url = ref_urls.get(marker_num)
        if ref_url and ref_url in doc_lookup:
            resolved[marker_num] = doc_lookup[ref_url]
        else:
            unresolved_markers.add(marker_num)

    unresolved_count = len(unresolved_markers)

    # Build new citation array sorted by original marker number
    sorted_markers = sorted(resolved.keys())
    # Mapping from old marker -> new 1-indexed marker
    old_to_new: dict[int, int] = {}
    citations: list[Citation] = []
    for new_idx, old_marker in enumerate(sorted_markers, start=1):
        doc = resolved[old_marker]
        old_to_new[old_marker] = new_idx
        citations.append(
            Citation(
                marker=new_idx,
                doc_id=doc.get("id", ""),
                url=doc.get("url", ""),
                title=doc.get("title", ""),
            )
        )

    # Replace markers in text
    def replace_marker(m: re.Match) -> str:
        num = int(m.group(1))
        if num in unresolved_markers:
            return ""  # Strip unresolved marker
        if num in old_to_new:
            new_num = old_to_new[num]
            return f"[{new_num}]"
        return ""  # Unknown marker, strip

    translated_text = _MARKER_RE.sub(replace_marker, text)

    return translated_text, citations, unresolved_count

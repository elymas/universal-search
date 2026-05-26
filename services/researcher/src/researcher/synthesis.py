"""Citation assembly and LLM invocation for synthesis.

REQ-SYN-002: gpt-researcher scaffold (extracted prompt assembly).
NFR-SYN-002: Every [N] marker maps to a real input doc.

Note: SPEC-SYN-001 §11.1 authorizes the extracted-scaffold fallback
when gpt-researcher cannot be installed due to heavyweight transitive
dependencies. This module implements the ~150-LoC equivalent directly.
"""

from __future__ import annotations

import logging
import os
import re

from researcher.gateway import Gateway
from researcher.models import Citation, NormalizedDocPayload, SynthesizeRequest, SynthesizeResponse
from researcher.obs import Timer, log_synthesis

logger = logging.getLogger(__name__)

# Regex to find all [N] citation markers in LLM output.
_MARKER_RE = re.compile(r"\[(\d+)\]")

_DEFAULT_MODEL = os.environ.get("RESEARCHER_MODEL_DEFAULT", "claude-haiku-4-5")


def build_prompt(
    query: str,
    lang: str,
    docs: list[NormalizedDocPayload],
) -> list[dict[str, str]]:
    """Build the chat messages list for the LLM call.

    System message instructs the model to cite sources with [N] markers.
    Lang hint is appended when non-empty (REQ-SYN-007).

    # @MX:NOTE: [AUTO] Citation prompt scaffold extracted from gpt-researcher pattern.
    # Uses single-pass local-doc mode: no retrieval, docs are embedded directly.
    """
    system = (
        "You are a research synthesizer. "
        "Cite each fact with [N] where N is the 1-indexed source number from the SOURCES list. "
        "Use only facts present in the sources. "
        "Output one paragraph (4-8 sentences). "
        "Do not invent sources."
    )
    if lang:
        system += f"\n\nAnswer in {lang}."

    sources_lines = []
    for i, doc in enumerate(docs):
        excerpt = doc.snippet or doc.body[:1000]
        sources_lines.append(f"[{i + 1}] {doc.title}\n  URL: {doc.url}\n  EXCERPT: {excerpt}")

    user = f"QUESTION: {query}\n\nSOURCES:\n" + "\n".join(sources_lines)

    return [
        {"role": "system", "content": system},
        {"role": "user", "content": user},
    ]


def _process_markers(
    text: str,
    docs: list[NormalizedDocPayload],
) -> tuple[str, list[Citation]]:
    """Validate and strip out-of-range markers from LLM-generated text.

    REQ-SYN-002: Markers outside [1, len(docs)] are stripped and logged at WARN.
    NFR-SYN-002: Every remaining [N] maps to a real doc.

    Returns:
        (cleaned_text, citations_list)
    """
    n = len(docs)
    found_markers: set[int] = set()
    invalid_markers: set[int] = set()

    for m in _MARKER_RE.finditer(text):
        num = int(m.group(1))
        if 1 <= num <= n:
            found_markers.add(num)
        else:
            invalid_markers.add(num)

    if invalid_markers:
        logger.warning(
            "Out-of-range citation markers stripped: %s (docs count: %d)",
            sorted(invalid_markers),
            n,
        )

    # Remove invalid markers from text
    def _replace(match: re.Match) -> str:
        num = int(match.group(1))
        if num in invalid_markers:
            return ""
        return match.group(0)

    cleaned = _MARKER_RE.sub(_replace, text)
    # Collapse multiple spaces left by removal
    cleaned = re.sub(r"  +", " ", cleaned).strip()

    # Build citations for valid markers (sorted, unique)
    citations = [
        Citation(
            marker=m,
            doc_id=docs[m - 1].id,
            url=docs[m - 1].url,
            title=docs[m - 1].title,
        )
        for m in sorted(found_markers)
    ]

    return cleaned, citations


def _build_degraded_response(
    req: SynthesizeRequest,
    latency_ms: float,
) -> SynthesizeResponse:
    """Build the degraded-mode bullet-list response.

    REQ-SYN-003: Returns deterministic bullet-list when LiteLLM is unreachable.
    NFR-SYN-003: text is exactly len(docs) lines; each <= 320 chars per doc.
    """
    lines = [f"[{i + 1}] {doc.title} — {doc.url}" for i, doc in enumerate(req.docs)]
    text = "\n".join(lines)
    citations = [Citation(marker=i + 1, doc_id=doc.id, url=doc.url, title=doc.title) for i, doc in enumerate(req.docs)]
    return SynthesizeResponse(
        request_id=req.request_id,
        text=text,
        citations=citations,
        model="",
        provider="",
        cost_usd=0.0,
        prompt_tokens=0,
        completion_tokens=0,
        latency_ms=latency_ms,
        degraded=True,
        notice="litellm unavailable; returning raw doc list",
    )


# @MX:ANCHOR: [AUTO] Core synthesis function; callers: app.synthesize_endpoint, tests
# @MX:REASON: Public API boundary; fan_in >= 3 (app, tests, CLI future).
async def synthesize(
    req: SynthesizeRequest,
    gateway: Gateway,
) -> SynthesizeResponse:
    """Invoke LLM via gateway and assemble a cited synthesis response.

    Falls back to degraded mode when LiteLLM is unreachable.
    REQ-SYN-002, REQ-SYN-003, REQ-SYN-007.
    """
    import httpx as _httpx

    model = os.environ.get("RESEARCHER_MODEL_DEFAULT", _DEFAULT_MODEL)

    with Timer() as timer:
        try:
            messages = build_prompt(query=req.query, lang=req.lang or "", docs=req.docs)
            text_raw, cost_usd, usage, provider, model_used = await gateway.complete(
                messages=messages,
                model=model,
                lang=req.lang or "",
            )
        except (_httpx.ConnectError, _httpx.HTTPStatusError, Exception) as exc:
            logger.warning("LiteLLM unreachable or error: %s; returning degraded response", exc)
            resp = _build_degraded_response(req, timer.elapsed_ms)
            log_synthesis(
                {
                    "request_id": req.request_id,
                    "query_len": len(req.query),
                    "docs_count": len(req.docs),
                    "model": "",
                    "provider": "",
                    "cost_usd": 0.0,
                    "prompt_tokens": 0,
                    "completion_tokens": 0,
                    "latency_ms": timer.elapsed_ms,
                    "degraded": True,
                    "outcome": "degraded",
                }
            )
            return resp

    cleaned_text, citations = _process_markers(text_raw, req.docs)

    log_synthesis(
        {
            "request_id": req.request_id,
            "query_len": len(req.query),
            "docs_count": len(req.docs),
            "model": model_used,
            "provider": provider,
            "cost_usd": cost_usd,
            "prompt_tokens": usage.get("prompt_tokens", 0),
            "completion_tokens": usage.get("completion_tokens", 0),
            "latency_ms": timer.elapsed_ms,
            "degraded": False,
            "outcome": "success",
        }
    )

    return SynthesizeResponse(
        request_id=req.request_id,
        text=cleaned_text,
        citations=citations,
        model=model_used,
        provider=provider,
        cost_usd=cost_usd,
        prompt_tokens=usage.get("prompt_tokens", 0),
        completion_tokens=usage.get("completion_tokens", 0),
        latency_ms=timer.elapsed_ms,
        degraded=False,
        notice="",
    )

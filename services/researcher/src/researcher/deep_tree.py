"""POST /decompose_query endpoint for SPEC-DEEP-003 Phase C.

REQ-DEEP3-003: Returns breadth sub-queries for tree expansion.
REQ-DEEP3-009a: Input validation with Pydantic.

Uses in-house decomposition prompt (no gpt-researcher dependency).
"""

from __future__ import annotations

import json
import logging
import os

import json_repair
from fastapi import APIRouter
from pydantic import BaseModel, ConfigDict, Field

from researcher.gateway import Gateway

logger = logging.getLogger(__name__)

router = APIRouter()

_DEFAULT_MODEL = os.environ.get("RESEARCHER_MODEL_DEFAULT", "claude-haiku-4-5")


class DecomposeRequest(BaseModel):
    """Request body for POST /decompose_query.

    REQ-DEEP3-009a: Pydantic v2 model with input validation.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    root_query: str
    parent_query: str
    parent_evidence_summary: str
    breadth: int = Field(ge=1, le=8)


class DecomposeResponse(BaseModel):
    """Response body for POST /decompose_query.

    REQ-DEEP3-003: Returns sub_queries list of strings.
    """

    model_config = ConfigDict(extra="forbid")

    sub_queries: list[str]


def build_decompose_prompt(
    root_query: str,
    parent_query: str,
    parent_evidence_summary: str,
    breadth: int,
) -> list[dict[str, str]]:
    """Build the chat messages for the LLM decomposition call.

    # @MX:NOTE: [AUTO] In-house decomposition prompt for tree exploration
    # @MX:SPEC: SPEC-DEEP-003
    """
    system = (
        "You are a research query decomposer. "
        "Generate diverse sub-queries that explore different aspects of a topic. "
        "Return ONLY a JSON array of strings, no other text."
    )

    user = (
        f"Given the root query: {root_query}\n"
        f"And the parent query: {parent_query}\n"
        f"With parent evidence: {parent_evidence_summary}\n"
        f"\n"
        f"Generate exactly {breadth} diverse sub-queries that explore different aspects.\n"
        f"Return as a JSON array of strings."
    )

    return [
        {"role": "system", "content": system},
        {"role": "user", "content": user},
    ]


def parse_sub_queries(raw: str, breadth: int) -> list[str]:
    """Parse an LLM decomposition response into sub-queries (SPEC-DEEP-003a).

    Three-tier cascade, each tier feeding ONE shared normalization block:
      1. standard  — json.loads(raw)
      2. substring — json.loads(raw[first '[' : last ']' + 1])
      3. repaired  — json_repair.loads(raw)  (tolerates trailing commas,
         single/unquoted keys, truncated output, prose/fence wrapping)
    Total failure returns ``[]`` (no exception escapes this function).

    REQ-DEEP3a-101..104: tier ordering, shared normalization, no-raise,
        tiers 1/2 byte-identical to the parent SPEC on already-passing input.
    REQ-DEEP3a-201/202: exactly one tier-attribution log per call
        (standard/substring/repaired/failed); repaired and failed at WARNING.
    REQ-DEEP3-003: truncate excess sub-queries beyond breadth.

    # @MX:NOTE: [AUTO] Three-tier parse cascade (standard -> substring ->
    #   repaired -> []) with a no-raise contract and single tier-attribution
    #   log line per call. fan_in=1 (sole caller: decompose_query).
    # @MX:SPEC: SPEC-DEEP-003a

    Args:
        raw: The raw LLM response text.
        breadth: Maximum number of sub-queries to return.

    Returns:
        List of sub-query strings, at most ``breadth`` items. Returns ``[]``
        on total parse failure or when the parsed value is not a usable list.
    """
    parsed, parse_tier = _parse_raw(raw)

    # Shared post-parse normalization (REQ-DEEP3a-102): identical for all
    # three tiers — list check, str coercion, drop falsy, breadth truncation.
    # A non-list value from any tier (including a non-list repair output) is a
    # failed outcome (REQ-DEEP3a-103).
    if not isinstance(parsed, list):
        logger.warning(
            "parse_sub_queries tier=failed: parsed value is not a list (type=%s, source_tier=%s): %s",
            type(parsed).__name__,
            parse_tier,
            raw[:200],
        )
        return []

    queries = [str(item) for item in parsed if item]

    if len(queries) > breadth:
        logger.warning(
            "LLM returned %d sub-queries, truncating to breadth=%d",
            len(queries),
            breadth,
        )
        queries = queries[:breadth]

    # Tier-attribution logging (REQ-DEEP3a-201/202): exactly one line per
    # call. The "failed" outcome is already logged above (non-list branch);
    # here only successful tiers reach this point: repaired at WARNING
    # (signals reasoning-model JSON drift), standard/substring at INFO.
    if parse_tier == "repaired":
        logger.warning("parse_sub_queries tier=repaired: recovered via json-repair: %s", raw[:200])
    elif parse_tier == "standard":
        logger.info("parse_sub_queries tier=standard")
    else:  # substring
        logger.info("parse_sub_queries tier=substring")

    return queries


def _parse_raw(raw: str) -> tuple[object, str]:
    """Run the three-tier parse cascade and return (value, tier-name).

    tier-name is one of: ``standard``, ``substring``, ``repaired``, ``failed``.
    On ``failed`` the returned value is ``None`` (caller normalizes to ``[]``).
    The repair tier is guarded so no exception escapes (REQ-DEEP3a-103).
    """
    # Tier 1: standard json.loads on the whole response.
    try:
        return json.loads(raw), "standard"
    except (json.JSONDecodeError, ValueError):
        pass

    # Tier 2: extract the array substring [..] and json.loads that slice.
    start = raw.find("[")
    end = raw.rfind("]") + 1
    if start >= 0 and end > start:
        try:
            return json.loads(raw[start:end]), "substring"
        except (json.JSONDecodeError, ValueError):
            pass

    # Tier 3: json_repair on the original raw (it strips prose/fences itself).
    # REQ-DEEP3a-103: guard defensively — any exception yields the failed tier.
    try:
        repaired = json_repair.loads(raw)
    except Exception:  # noqa: BLE001 — broad on purpose; repair tier must not raise
        return None, "failed"

    # json_repair returns None/empty for unrecoverable input; treat as failure
    # so the caller returns [] (REQ-DEEP3a-103).
    if repaired is None:
        return None, "failed"

    return repaired, "repaired"


# @MX:ANCHOR: [AUTO] Decompose endpoint; callers: app router, tests
# @MX:REASON: Public API boundary for /deep tree exploration; fan_in >= 3 (app, tests, Go HTTP client)
# @MX:SPEC: SPEC-DEEP-003
@router.post("/decompose_query", response_model=DecomposeResponse)
async def decompose_query(req: DecomposeRequest) -> DecomposeResponse:
    """POST /decompose_query -- generate sub-queries for tree expansion.

    REQ-DEEP3-003: Returns up to breadth sub-queries.
    REQ-DEEP3-009a: Input validated by Pydantic (breadth in [1, 8]).
    """
    model = os.environ.get("RESEARCHER_MODEL_DEFAULT", _DEFAULT_MODEL)
    gateway = Gateway()

    messages = build_decompose_prompt(
        root_query=req.root_query,
        parent_query=req.parent_query,
        parent_evidence_summary=req.parent_evidence_summary,
        breadth=req.breadth,
    )

    raw_text, _, _, _, _ = await gateway.complete(
        messages=messages,
        model=model,
        lang="",
    )

    sub_queries = parse_sub_queries(raw_text, req.breadth)

    return DecomposeResponse(sub_queries=sub_queries)

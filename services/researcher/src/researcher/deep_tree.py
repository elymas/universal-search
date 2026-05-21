"""POST /decompose_query endpoint for SPEC-DEEP-003 Phase C.

REQ-DEEP3-003: Returns breadth sub-queries for tree expansion.
REQ-DEEP3-009a: Input validation with Pydantic.

Uses in-house decomposition prompt (no gpt-researcher dependency).
"""

from __future__ import annotations

import json
import logging
import os

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
    """Parse LLM response into sub-queries, truncating to breadth.

    REQ-DEEP3-003: Truncates excess sub-queries beyond breadth.

    Returns:
        List of sub-query strings, at most breadth items.
    """
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        # Try to extract JSON array from response text
        start = raw.find("[")
        end = raw.rfind("]") + 1
        if start >= 0 and end > start:
            try:
                parsed = json.loads(raw[start:end])
            except json.JSONDecodeError:
                logger.warning("Failed to parse LLM response as JSON: %s", raw[:200])
                return []
        else:
            logger.warning("No JSON array found in LLM response: %s", raw[:200])
            return []

    if not isinstance(parsed, list):
        logger.warning("LLM response is not a list: %s", type(parsed).__name__)
        return []

    # Filter to strings only
    queries = [str(item) for item in parsed if item]

    if len(queries) > breadth:
        logger.warning(
            "LLM returned %d sub-queries, truncating to breadth=%d",
            len(queries),
            breadth,
        )
        queries = queries[:breadth]

    return queries


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

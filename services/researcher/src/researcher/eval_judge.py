"""DeepEval faithfulness judge endpoint for SPEC-EVAL-001 REQ-EVAL1-004.

Exposes POST /judge/faithfulness. For each claim, an LLM judge decides whether
the cited document bodies entail the claim. The aggregate faithfulness score is
supported_claims / total_claims (HISTORY D3).

Determinism (FROZEN per HISTORY D7 / NFR-EVAL1-001): the judge is invoked via
LiteLLM with temperature=0, top_p=1, seed=42. The judge model is selected by
the EVAL_JUDGE_MODEL env var (default claude-haiku-4-5, HISTORY D4) and routed
exclusively through LiteLLM (NFR-EVAL1-005).

The judge function is injectable so unit tests run without deepeval/LiteLLM.
The default judge lazily wraps DeepEval's FaithfulnessMetric (imported only when
first called) so the module imports cleanly even when deepeval is absent.
"""

from __future__ import annotations

import os
from typing import Callable

from fastapi import APIRouter
from pydantic import BaseModel, ConfigDict

# A Judge decides (supported, rationale) for a claim given its cited doc bodies.
Judge = Callable[[str, list[str]], tuple[bool, str]]

# EVAL_JUDGE_MODEL default judge model (HISTORY D4).
DEFAULT_JUDGE_MODEL = "claude-haiku-4-5"


class ClaimInput(BaseModel):
    """One segmented claim plus the doc IDs it cites."""

    model_config = ConfigDict(extra="forbid")

    text: str
    cited_doc_ids: list[str] = []


class JudgeRequest(BaseModel):
    """POST /judge/faithfulness request body (REQ-EVAL1-004)."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    query_id: str
    claims: list[ClaimInput] = []
    corpus: dict[str, str] = {}


class ClaimScore(BaseModel):
    """Per-claim judge verdict."""

    model_config = ConfigDict(extra="forbid")

    text: str
    supported: bool
    judge_rationale: str


class JudgeResponse(BaseModel):
    """POST /judge/faithfulness response body (REQ-EVAL1-004)."""

    model_config = ConfigDict(extra="forbid")

    query_id: str
    claim_scores: list[ClaimScore]
    faithfulness_score: float
    total_claims: int
    supported_claims: int


def deterministic_litellm_params(model: str) -> dict[str, object]:
    """Return the FROZEN deterministic LiteLLM params (NFR-EVAL1-001, HISTORY D7).

    temperature=0, top_p=1, seed=42 are pinned at the SPEC level and may not be
    altered without a constitution amendment.
    """
    return {"model": model, "temperature": 0, "top_p": 1, "seed": 42}


def _resolve_bodies(claim: ClaimInput, corpus: dict[str, str]) -> list[str]:
    """Return the corpus bodies for the docs the claim cites (cited-only scope).

    REQ-EVAL1-005(c): only the docs the claim actually cites are judged.
    """
    return [corpus[d] for d in claim.cited_doc_ids if d in corpus]


def make_router(judge: Judge) -> APIRouter:
    """Build the /judge/faithfulness router bound to the given judge function.

    Injecting the judge keeps the endpoint testable without deepeval/LiteLLM.
    """
    router = APIRouter()

    @router.post("/judge/faithfulness", response_model=JudgeResponse)
    async def judge_faithfulness(req: JudgeRequest) -> JudgeResponse:  # noqa: D401
        scores: list[ClaimScore] = []
        supported = 0
        for claim in req.claims:
            bodies = _resolve_bodies(claim, req.corpus)
            is_supported, rationale = judge(claim.text, bodies)
            if is_supported:
                supported += 1
            scores.append(
                ClaimScore(text=claim.text, supported=is_supported, judge_rationale=rationale)
            )

        total = len(req.claims)
        # Vacuously faithful when there are no claims (avoid divide-by-zero).
        score = 1.0 if total == 0 else supported / total
        return JudgeResponse(
            query_id=req.query_id,
            claim_scores=scores,
            faithfulness_score=score,
            total_claims=total,
            supported_claims=supported,
        )

    return router


def deepeval_judge(model: str | None = None) -> Judge:
    """Return a Judge backed by DeepEval's FaithfulnessMetric via LiteLLM.

    deepeval + litellm are imported lazily so this module loads without them.
    The judge runs each claim through FaithfulnessMetric with the deterministic
    params; a claim is supported iff the metric score for that claim is >= 0.5.
    """
    judge_model = model or os.environ.get("EVAL_JUDGE_MODEL", DEFAULT_JUDGE_MODEL)
    params = deterministic_litellm_params(judge_model)

    def _judge(claim_text: str, cited_bodies: list[str]) -> tuple[bool, str]:
        # Lazy imports: only required when the real judge is actually invoked.
        from deepeval.metrics import FaithfulnessMetric  # type: ignore
        from deepeval.models import LiteLLMModel  # type: ignore
        from deepeval.test_case import LLMTestCase  # type: ignore

        llm = LiteLLMModel(
            model=judge_model,
            temperature=params["temperature"],
            top_p=params["top_p"],
            seed=params["seed"],
        )
        metric = FaithfulnessMetric(model=llm, include_reason=True)
        test_case = LLMTestCase(
            input=claim_text,
            actual_output=claim_text,
            retrieval_context=cited_bodies or [""],
        )
        metric.measure(test_case)
        supported = bool(metric.score is not None and metric.score >= 0.5)
        return supported, metric.reason or ""

    return _judge

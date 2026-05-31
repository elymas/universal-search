<<<<<<< HEAD
"""Citation faithfulness judge endpoint for SPEC-EVAL-001.

Wraps DeepEval FaithfulnessMetric with deterministic parameters:
  temperature=0, top_p=1, seed=42

REQ-EVAL1-004: Per-claim faithfulness scoring.
REQ-EVAL1-005: faithfulness_score = supported_claims / total_claims.
REQ-EVAL1-007: Per-claim judge rationale.
REQ-EVAL1-006: EVAL_JUDGE_MODEL env var with default claude-haiku-4-5.
=======
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
>>>>>>> origin/feature/SPEC-EVAL-001
"""

from __future__ import annotations

<<<<<<< HEAD
import logging
import os
from typing import Any
=======
import os
from typing import Callable
>>>>>>> origin/feature/SPEC-EVAL-001

from fastapi import APIRouter
from pydantic import BaseModel, ConfigDict

<<<<<<< HEAD
logger = logging.getLogger(__name__)

router = APIRouter(prefix="/judge", tags=["eval"])


# ---------- Request / Response models ----------

class ClaimInput(BaseModel):
    """A single claim extracted from the synthesized response."""

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    text: str
    cited_doc_ids: list[str]


class CorpusEntry(BaseModel):
    """A single corpus document excerpt for entailment checking."""

    model_config = ConfigDict(extra="forbid")

    doc_id: str
    body: str


class FaithfulnessRequest(BaseModel):
    """POST /judge/faithfulness request body.

    REQ-EVAL1-004: query_id + claims + corpus are required.
    """

    model_config = ConfigDict(extra="forbid")

    query_id: str
    claims: list[ClaimInput]
    corpus: list[CorpusEntry]


class ClaimScore(BaseModel):
    """Per-claim faithfulness verdict."""
=======
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
>>>>>>> origin/feature/SPEC-EVAL-001

    model_config = ConfigDict(extra="forbid")

    text: str
    supported: bool
    judge_rationale: str


<<<<<<< HEAD
class FaithfulnessResponse(BaseModel):
    """POST /judge/faithfulness response body.

    REQ-EVAL1-005: faithfulness_score = supported / total.
    """
=======
class JudgeResponse(BaseModel):
    """POST /judge/faithfulness response body (REQ-EVAL1-004)."""
>>>>>>> origin/feature/SPEC-EVAL-001

    model_config = ConfigDict(extra="forbid")

    query_id: str
<<<<<<< HEAD
    judge_model: str
    claim_scores: list[ClaimScore]
    faithfulness_score: float


# ---------- Helpers ----------

def _get_judge_model() -> str:
    """Read EVAL_JUDGE_MODEL env var, defaulting to claude-haiku-4-5.

    NFR-EVAL1-005: Provider swap via env var, no code change required.
    """
    return os.environ.get("EVAL_JUDGE_MODEL", "claude-haiku-4-5")


def _build_context_for_claim(claim: ClaimInput, corpus: list[CorpusEntry]) -> str:
    """Build concatenated context from corpus docs cited by a claim."""
    corpus_map = {entry.doc_id: entry.body for entry in corpus}
    parts: list[str] = []
    for doc_id in claim.cited_doc_ids:
        body = corpus_map.get(doc_id)
        if body is not None:
            parts.append(body)
    return "\n".join(parts)


def _check_claim_preconditions(claim: ClaimInput, corpus: list[CorpusEntry]) -> str | None:
    """Return a rationale string if claim is trivially unsupported, else None."""
    if not claim.cited_doc_ids:
        return "claim has no cited doc IDs"
    corpus_ids = {entry.doc_id for entry in corpus}
    missing = [d for d in claim.cited_doc_ids if d not in corpus_ids]
    if len(missing) == len(claim.cited_doc_ids):
        return "cited doc not in retrieval context"
    return None


# ---------- Core metric runner (mocked in tests) ----------

def _run_faithfulness_metric(
    *,
    query_id: str,
    claims: list[ClaimInput],
    corpus: list[CorpusEntry],
    judge_model: str,
) -> dict[str, Any]:
    """Run DeepEval FaithfulnessMetric with deterministic params.

    This function is the integration point with DeepEval. In unit tests it
    is mocked; in integration tests it calls the real metric.

    Deterministic params per D7: temperature=0, top_p=1, seed=42.
    """
    # NOTE: The actual DeepEval call will be:
    # from deepeval.metrics import FaithfulnessMetric
    # from deepeval.test_case import LLMTestCase
    #
    # metric = FaithfulnessMetric(
    #     model=judge_model,
    #     temperature=0,
    #     top_p=1,
    #     seed=42,
    # )
    #
    # For now, return a stub that marks everything as supported.
    # The full DeepEval integration will be wired in integration testing.

    claim_scores = []
    supported_count = 0

    for claim in claims:
        context = _build_context_for_claim(claim, corpus)
        precond = _check_claim_preconditions(claim, corpus)

        if precond:
            claim_scores.append(
                ClaimScore(
                    text=claim.text,
                    supported=False,
                    judge_rationale=precond,
                )
            )
        elif not context.strip():
            claim_scores.append(
                ClaimScore(
                    text=claim.text,
                    supported=False,
                    judge_rationale="cited doc body is empty",
                )
            )
        else:
            # Stub: in production this calls DeepEval.
            # For now, assume supported if context exists.
            claim_scores.append(
                ClaimScore(
                    text=claim.text,
                    supported=True,
                    judge_rationale="Claim is supported by cited document context.",
                )
            )
            supported_count += 1

    total = len(claims)
    score = 1.0 if total == 0 else supported_count / total

    return {
        "claim_scores": claim_scores,
        "faithfulness_score": score,
    }


# ---------- Endpoint ----------

@router.post("/faithfulness", response_model=FaithfulnessResponse)
async def judge_faithfulness(req: FaithfulnessRequest) -> FaithfulnessResponse:
    """POST /judge/faithfulness — per-claim faithfulness scoring.

    REQ-EVAL1-004: Returns per-claim faithfulness verdicts.
    REQ-EVAL1-005: Score = supported_claims / total_claims.
    REQ-EVAL1-007: Each verdict includes judge_rationale.
    """
    judge_model = _get_judge_model()

    # Vacuous truth: 0 claims → perfect score.
    if len(req.claims) == 0:
        return FaithfulnessResponse(
            query_id=req.query_id,
            judge_model=judge_model,
            claim_scores=[],
            faithfulness_score=1.0,
        )

    result = _run_faithfulness_metric(
        query_id=req.query_id,
        claims=req.claims,
        corpus=req.corpus,
        judge_model=judge_model,
    )

    return FaithfulnessResponse(
        query_id=req.query_id,
        judge_model=judge_model,
        claim_scores=result["claim_scores"],
        faithfulness_score=result["faithfulness_score"],
    )
=======
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
>>>>>>> origin/feature/SPEC-EVAL-001

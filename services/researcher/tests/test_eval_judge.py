"""Tests for the DeepEval faithfulness judge endpoint (SPEC-EVAL-001 REQ-EVAL1-004).

The real DeepEval FaithfulnessMetric judges via an LLM and cannot be unit-tested
deterministically, so these tests inject a stub judge. The determinism contract
(temperature=0, top_p=1, seed=42) is asserted on the LiteLLM params builder.
"""

from __future__ import annotations

from fastapi.testclient import TestClient

from researcher.eval_judge import (
    ClaimInput,
    JudgeRequest,
    deterministic_litellm_params,
    make_router,
)


def _client(judge):
    """Build a FastAPI test client mounting the judge router with a stub judge."""
    from fastapi import FastAPI

    app = FastAPI()
    app.include_router(make_router(judge))
    return TestClient(app)


def test_judge_endpoint_returns_per_claim_scores():
    """REQ-EVAL1-004: endpoint returns per-claim supported verdicts + rationale."""

    def stub_judge(claim_text, cited_bodies):
        # Support the claim iff any cited body contains the word "supported".
        supported = any("supported" in b for b in cited_bodies)
        return supported, "matched" if supported else "no entailment"

    client = _client(stub_judge)
    payload = {
        "query_id": "EVAL-001-Q001",
        "claims": [
            {"text": "claim one [1]", "cited_doc_ids": ["doc-001"]},
            {"text": "claim two [2]", "cited_doc_ids": ["doc-002"]},
        ],
        "corpus": {"doc-001": "this is supported", "doc-002": "unrelated text"},
    }
    resp = client.post("/judge/faithfulness", json=payload)
    assert resp.status_code == 200
    body = resp.json()
    assert body["query_id"] == "EVAL-001-Q001"
    assert body["total_claims"] == 2
    assert body["supported_claims"] == 1
    assert len(body["claim_scores"]) == 2
    assert body["claim_scores"][0]["supported"] is True
    assert body["claim_scores"][1]["supported"] is False
    assert body["claim_scores"][1]["judge_rationale"]


def test_judge_score_formula():
    """REQ-EVAL1-004: faithfulness_score == supported_claims / total_claims."""

    def stub_judge(claim_text, cited_bodies):
        return ("yes" in claim_text), "r"

    client = _client(stub_judge)
    payload = {
        "query_id": "Q",
        "claims": [
            {"text": "yes a [1]", "cited_doc_ids": ["d"]},
            {"text": "yes b [1]", "cited_doc_ids": ["d"]},
            {"text": "no c [1]", "cited_doc_ids": ["d"]},
            {"text": "no d [1]", "cited_doc_ids": ["d"]},
        ],
        "corpus": {"d": "body"},
    }
    body = client.post("/judge/faithfulness", json=payload).json()
    assert body["total_claims"] == 4
    assert body["supported_claims"] == 2
    assert abs(body["faithfulness_score"] - 0.5) < 1e-9


def test_judge_empty_claims_scores_one():
    """No claims → vacuously faithful (score 1.0), not a divide-by-zero."""

    client = _client(lambda c, b: (True, "r"))
    body = client.post(
        "/judge/faithfulness",
        json={"query_id": "Q", "claims": [], "corpus": {}},
    ).json()
    assert body["total_claims"] == 0
    assert body["faithfulness_score"] == 1.0


def test_judge_uses_deterministic_params():
    """REQ-EVAL1-004 / NFR-EVAL1-001: temperature=0, top_p=1, seed=42 are FROZEN."""
    params = deterministic_litellm_params("claude-haiku-4-5")
    assert params["temperature"] == 0
    assert params["top_p"] == 1
    assert params["seed"] == 42
    assert params["model"] == "claude-haiku-4-5"


def test_judge_request_model_validates():
    """JudgeRequest is a Pydantic model with the expected shape."""
    req = JudgeRequest(
        query_id="Q",
        claims=[ClaimInput(text="c", cited_doc_ids=["d"])],
        corpus={"d": "body"},
    )
    assert req.query_id == "Q"
    assert req.claims[0].cited_doc_ids == ["d"]


def test_judge_resolves_cited_bodies_only():
    """The judge receives only the bodies of the docs a claim cites."""
    received = {}

    def stub_judge(claim_text, cited_bodies):
        received["bodies"] = cited_bodies
        return True, "r"

    client = _client(stub_judge)
    payload = {
        "query_id": "Q",
        "claims": [{"text": "c [1]", "cited_doc_ids": ["doc-001"]}],
        "corpus": {"doc-001": "body one", "doc-002": "body two"},
    }
    client.post("/judge/faithfulness", json=payload)
    # Only doc-001's body should reach the judge, not doc-002.
    assert received["bodies"] == ["body one"]

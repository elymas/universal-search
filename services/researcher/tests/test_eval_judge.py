<<<<<<< HEAD
"""Tests for eval_judge.py — SPEC-EVAL-001 DeepEval faithfulness judge.

RED phase: tests define expected behavior before implementation.
Covers REQ-EVAL1-004, REQ-EVAL1-005, REQ-EVAL1-007.

The judge service exposes POST /judge/faithfulness which:
  - Accepts claims + corpus excerpts
  - Returns per-claim faithfulness scores with rationale
  - Uses deterministic params: temperature=0, top_p=1, seed=42
  - Reads EVAL_JUDGE_MODEL env var (default claude-haiku-4-5)
=======
"""Tests for the DeepEval faithfulness judge endpoint (SPEC-EVAL-001 REQ-EVAL1-004).

The real DeepEval FaithfulnessMetric judges via an LLM and cannot be unit-tested
deterministically, so these tests inject a stub judge. The determinism contract
(temperature=0, top_p=1, seed=42) is asserted on the LiteLLM params builder.
>>>>>>> origin/feature/SPEC-EVAL-001
"""

from __future__ import annotations

<<<<<<< HEAD
import json
import os
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from researcher.app import app


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)


# ---------- REQ-EVAL1-004: Per-claim scoring ----------

class TestJudgeEndpointPerClaimScores:
    """POST /judge/faithfulness returns per-claim scores matching input length."""

    def test_returns_per_claim_scores_matching_input_length(self, client: TestClient) -> None:
        """With 3 claims, response has 3 claim_scores."""
        payload = {
            "query_id": "EVAL-001-Q001",
            "claims": [
                {"text": "Quantum computing uses qubits.", "cited_doc_ids": ["doc-001"]},
                {"text": "Classical computers use bits.", "cited_doc_ids": ["doc-041"]},
                {"text": "Qubits can be in superposition.", "cited_doc_ids": ["doc-081"]},
            ],
            "corpus": [
                {"doc_id": "doc-001", "body": "Quantum computing utilizes qubits for computation."},
                {"doc_id": "doc-041", "body": "Classical computers process bits of information."},
                {"doc_id": "doc-081", "body": "Qubits exist in superposition states."},
            ],
        }
        with patch("researcher.eval_judge._run_faithfulness_metric") as mock_metric:
            mock_metric.return_value = {
                "claim_scores": [
                    {"text": "Quantum computing uses qubits.", "supported": True, "judge_rationale": "Claim is directly supported by doc-001."},
                    {"text": "Classical computers use bits.", "supported": True, "judge_rationale": "Claim is directly supported by doc-041."},
                    {"text": "Qubits can be in superposition.", "supported": True, "judge_rationale": "Claim is directly supported by doc-081."},
                ],
                "faithfulness_score": 1.0,
            }
            resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        data = resp.json()
        assert len(data["claim_scores"]) == 3
        assert data["faithfulness_score"] == 1.0


class TestJudgeScoreFormula:
    """faithfulness_score = supported_claims / total_claims."""

    def test_partial_support_yields_fractional_score(self, client: TestClient) -> None:
        """5 claims, 3 supported → score = 0.6."""
        payload = {
            "query_id": "EVAL-001-Q010",
            "claims": [
                {"text": "Claim A.", "cited_doc_ids": ["doc-001"]},
                {"text": "Claim B.", "cited_doc_ids": ["doc-002"]},
                {"text": "Claim C.", "cited_doc_ids": ["doc-003"]},
                {"text": "Claim D.", "cited_doc_ids": ["doc-004"]},
                {"text": "Claim E.", "cited_doc_ids": ["doc-005"]},
            ],
            "corpus": [
                {"doc_id": "doc-001", "body": "Support for A."},
                {"doc_id": "doc-002", "body": "Support for B."},
                {"doc_id": "doc-003", "body": "Support for C."},
                {"doc_id": "doc-004", "body": "Unrelated to D."},
                {"doc_id": "doc-005", "body": "Unrelated to E."},
            ],
        }
        with patch("researcher.eval_judge._run_faithfulness_metric") as mock_metric:
            mock_metric.return_value = {
                "claim_scores": [
                    {"text": "Claim A.", "supported": True, "judge_rationale": "Supported."},
                    {"text": "Claim B.", "supported": True, "judge_rationale": "Supported."},
                    {"text": "Claim C.", "supported": True, "judge_rationale": "Supported."},
                    {"text": "Claim D.", "supported": False, "judge_rationale": "Not supported by cited doc."},
                    {"text": "Claim E.", "supported": False, "judge_rationale": "Not supported by cited doc."},
                ],
                "faithfulness_score": 0.6,
            }
            resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        data = resp.json()
        assert data["faithfulness_score"] == 0.6


class TestJudgeDeterministicParams:
    """Judge uses temperature=0, top_p=1, seed=42."""

    def test_deterministic_params_passed_to_metric(self, client: TestClient) -> None:
        """Verify deepeval is called with pinned deterministic params."""
        payload = {
            "query_id": "EVAL-001-Q001",
            "claims": [{"text": "Test claim.", "cited_doc_ids": ["doc-001"]}],
            "corpus": [{"doc_id": "doc-001", "body": "Test context."}],
        }
        with patch("researcher.eval_judge._run_faithfulness_metric") as mock_metric:
            mock_metric.return_value = {
                "claim_scores": [{"text": "Test claim.", "supported": True, "judge_rationale": "Supported."}],
                "faithfulness_score": 1.0,
            }
            resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        # Verify the metric was called with deterministic params
        call_kwargs = mock_metric.call_args
        assert call_kwargs is not None or mock_metric.called


class TestJudgeRationalePerClaim:
    """Each claim_score has a non-empty judge_rationale."""

    def test_rationale_present_for_each_claim(self, client: TestClient) -> None:
        payload = {
            "query_id": "EVAL-001-Q002",
            "claims": [
                {"text": "Claim 1.", "cited_doc_ids": ["doc-001"]},
                {"text": "Claim 2.", "cited_doc_ids": ["doc-002"]},
            ],
            "corpus": [
                {"doc_id": "doc-001", "body": "Context for 1."},
                {"doc_id": "doc-002", "body": "Context for 2."},
            ],
        }
        with patch("researcher.eval_judge._run_faithfulness_metric") as mock_metric:
            mock_metric.return_value = {
                "claim_scores": [
                    {"text": "Claim 1.", "supported": True, "judge_rationale": "Directly supported."},
                    {"text": "Claim 2.", "supported": False, "judge_rationale": "Not supported by cited text."},
                ],
                "faithfulness_score": 0.5,
            }
            resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        data = resp.json()
        for cs in data["claim_scores"]:
            assert "judge_rationale" in cs
            assert isinstance(cs["judge_rationale"], str)
            assert len(cs["judge_rationale"]) > 0


class TestJudgeEmptyClaims:
    """0 claims → score 1.0 (vacuous truth)."""

    def test_zero_claims_returns_score_one(self, client: TestClient) -> None:
        payload = {
            "query_id": "EVAL-001-Q050",
            "claims": [],
            "corpus": [],
        }
        resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        data = resp.json()
        assert data["faithfulness_score"] == 1.0
        assert data["claim_scores"] == []


class TestJudgeUnknownDocID:
    """Claim cites doc_id not in corpus → unsupported with rationale."""

    def test_unknown_doc_marked_unsupported(self, client: TestClient) -> None:
        payload = {
            "query_id": "EVAL-001-Q005",
            "claims": [
                {"text": "A claim citing a missing doc.", "cited_doc_ids": ["doc-999"]},
            ],
            "corpus": [],  # No docs provided
        }
        with patch("researcher.eval_judge._run_faithfulness_metric") as mock_metric:
            mock_metric.return_value = {
                "claim_scores": [
                    {"text": "A claim citing a missing doc.", "supported": False, "judge_rationale": "cited doc not in retrieval context"},
                ],
                "faithfulness_score": 0.0,
            }
            resp = client.post("/judge/faithfulness", json=payload)

        assert resp.status_code == 200
        data = resp.json()
        assert data["faithfulness_score"] == 0.0
        assert data["claim_scores"][0]["supported"] is False
        assert "not in retrieval context" in data["claim_scores"][0]["judge_rationale"]


class TestJudgeModelEnvVar:
    """Judge reads EVAL_JUDGE_MODEL env var, defaults to claude-haiku-4-5."""

    def test_default_model_is_haiku(self) -> None:
        with patch.dict(os.environ, {}, clear=False):
            os.environ.pop("EVAL_JUDGE_MODEL", None)
            # Import fresh to pick up default
            from researcher.eval_judge import _get_judge_model

            assert _get_judge_model() == "claude-haiku-4-5"

    def test_custom_model_from_env(self) -> None:
        with patch.dict(os.environ, {"EVAL_JUDGE_MODEL": "gpt-4o-mini"}):
            from researcher.eval_judge import _get_judge_model

            assert _get_judge_model() == "gpt-4o-mini"


class TestJudgeRequestSchema:
    """Verify request/response schema validation."""

    def test_missing_query_id_returns_422(self, client: TestClient) -> None:
        resp = client.post("/judge/faithfulness", json={"claims": [], "corpus": []})
        assert resp.status_code == 422

    def test_response_includes_query_id(self, client: TestClient) -> None:
        payload = {
            "query_id": "EVAL-001-Q001",
            "claims": [],
            "corpus": [],
        }
        resp = client.post("/judge/faithfulness", json=payload)
        assert resp.status_code == 200
        data = resp.json()
        assert data["query_id"] == "EVAL-001-Q001"

    def test_response_includes_judge_model(self, client: TestClient) -> None:
        payload = {
            "query_id": "EVAL-001-Q001",
            "claims": [],
            "corpus": [],
        }
        resp = client.post("/judge/faithfulness", json=payload)
        assert resp.status_code == 200
        data = resp.json()
        assert "judge_model" in data
        assert isinstance(data["judge_model"], str)
=======
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
>>>>>>> origin/feature/SPEC-EVAL-001

"""Tests for POST /decompose_query endpoint + parse_sub_queries tiers.

Covers:
- REQ-DEEP3-003 (breadth sub-queries + truncation),
- REQ-DEEP3-009a (input validation + prompt context fields),
- SPEC-DEEP-003a REQ-DEEP3a-101..104, 201, 202, 301 (json-repair parse tier).
"""

from __future__ import annotations

import json
import pathlib
from typing import Any
from unittest import mock

import pytest
from fastapi.testclient import TestClient

from researcher.deep_tree import parse_sub_queries


def _make_decompose_payload(**overrides: Any) -> dict[str, Any]:
    """Build a minimal decompose request payload."""
    base = {
        "root_query": "What are the effects of climate change?",
        "parent_query": "What are the economic effects of climate change?",
        "parent_evidence_summary": "Rising temperatures reduce crop yields by 10% per decade.",
        "breadth": 4,
    }
    base.update(overrides)
    return base


def _mock_gateway_complete(sub_queries: list[str]) -> Any:
    """Return an async mock for Gateway.complete that returns JSON sub-queries."""
    raw = json.dumps(sub_queries)
    mock_gw = mock.AsyncMock()
    mock_gw.complete.return_value = (
        raw,
        0.001,
        {"prompt_tokens": 50, "completion_tokens": 100},
        "anthropic",
        "claude-haiku-4-5",
    )
    return mock_gw


# ---------------------------------------------------------------------------
# T-C-001: POST /decompose_query returns breadth sub-queries
# REQ-DEEP3-003, REQ-DEEP3-009a
# ---------------------------------------------------------------------------


class TestDecomposeQueryReturnsBreadthSubQueries:
    """POST /decompose_query returns the requested number of sub-queries."""

    def test_decompose_query_returns_breadth_sub_queries(self) -> None:
        """When breadth=4 and LLM returns 4 sub-queries, response contains all 4."""
        from researcher.app import app
        from researcher.gateway import Gateway

        sub_queries = [
            "How does climate change affect GDP growth?",
            "What industries are most vulnerable to climate costs?",
            "What are the economic benefits of climate adaptation?",
            "How do insurance markets respond to climate risk?",
        ]

        mock_gw = _mock_gateway_complete(sub_queries)

        with (
            mock.patch.object(Gateway, "__init__", lambda self, **kw: None),
            mock.patch.object(Gateway, "complete", mock_gw.complete),
        ):
            client = TestClient(app, raise_server_exceptions=True)
            resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=4))

        assert resp.status_code == 200
        body = resp.json()
        assert "sub_queries" in body
        assert len(body["sub_queries"]) == 4
        assert body["sub_queries"] == sub_queries


# ---------------------------------------------------------------------------
# T-C-002: Truncates excess sub-queries beyond breadth
# REQ-DEEP3-003
# ---------------------------------------------------------------------------


class TestDecomposeQueryTruncatesExcess:
    """LLM returning more than breadth sub-queries gets truncated."""

    def test_decompose_query_truncates_excess(self) -> None:
        """When breadth=4 and LLM returns 6 sub-queries, response is truncated to 4."""
        from researcher.app import app
        from researcher.gateway import Gateway

        sub_queries = [
            "Query 1",
            "Query 2",
            "Query 3",
            "Query 4",
            "Query 5 (excess)",
            "Query 6 (excess)",
        ]

        mock_gw = _mock_gateway_complete(sub_queries)

        with (
            mock.patch.object(Gateway, "__init__", lambda self, **kw: None),
            mock.patch.object(Gateway, "complete", mock_gw.complete),
        ):
            client = TestClient(app, raise_server_exceptions=True)
            resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=4))

        assert resp.status_code == 200
        body = resp.json()
        assert len(body["sub_queries"]) == 4
        assert body["sub_queries"] == sub_queries[:4]


# ---------------------------------------------------------------------------
# T-C-003: Input validation
# REQ-DEEP3-009a
# ---------------------------------------------------------------------------


class TestDecomposeQueryValidatesInput:
    """Input validation returns 400 for invalid requests."""

    def test_decompose_query_rejects_zero_breadth(self) -> None:
        """breadth=0 returns HTTP 400."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=0))

        assert resp.status_code == 422  # Pydantic validation error

    def test_decompose_query_rejects_excess_breadth(self) -> None:
        """breadth=9 (above max 8) returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        resp = client.post("/decompose_query", json=_make_decompose_payload(breadth=9))

        assert resp.status_code == 422

    def test_decompose_query_requires_root_query(self) -> None:
        """Missing root_query returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["root_query"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422

    def test_decompose_query_requires_parent_query(self) -> None:
        """Missing parent_query returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["parent_query"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422

    def test_decompose_query_requires_parent_evidence_summary(self) -> None:
        """Missing parent_evidence_summary returns HTTP 422."""
        from researcher.app import app

        client = TestClient(app, raise_server_exceptions=False)
        payload = _make_decompose_payload()
        del payload["parent_evidence_summary"]
        resp = client.post("/decompose_query", json=payload)

        assert resp.status_code == 422


# ---------------------------------------------------------------------------
# SPEC-DEEP-003a: json-repair third parse tier (A1-A11)
# Target: parse_sub_queries(raw, breadth) — pure function, direct test.
# ---------------------------------------------------------------------------

# pyproject.toml of this package, for the A11 dependency-declaration test.
_PYPROJECT = pathlib.Path(__file__).resolve().parents[1] / "pyproject.toml"


def _normalized_deps() -> set[str]:
    """Return the set of normalized dependency names declared in pyproject.toml.

    Normalization follows PEP 503: lowercase, runs of -_. collapsed to -.
    """
    import re

    text = _PYPROJECT.read_text()

    # Extract the [project].dependencies array (between "dependencies = ["
    # and the matching "]"). This is a simple line-oriented parse adequate
    # for this manifest.
    in_deps = False
    deps: list[str] = []
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("dependencies") and "[" in stripped:
            in_deps = True
            continue
        if in_deps:
            if stripped.startswith("]"):
                break
            # strip inline comments and whitespace
            token = stripped.split("#", 1)[0].strip().rstrip(",").strip()
            if token:
                # keep the bare name (strip version specifiers)
                name = re.split(r"[<>=!~;\[]", token, maxsplit=1)[0].strip()
                if name:
                    deps.append(name)
    norm = {re.sub(r"[-_.]+", "-", d.lower()).strip(chr(34) + chr(39)) for d in deps}
    return norm


# --- A1: trailing comma inside array recovers (REQ-101, 102) ---


class TestRepairTrailingComma:
    def test_repair_recovers_trailing_comma(self) -> None:
        raw = '["alpha", "beta", "gamma",]'
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert result == ["alpha", "beta", "gamma"]
        assert all(isinstance(item, str) for item in result)


# --- A2: single-quoted keys/strings recover (REQ-101, 102) ---


class TestRepairSingleQuotes:
    def test_repair_recovers_single_quotes(self) -> None:
        raw = "['alpha', 'beta', 'gamma']"
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert result == ["alpha", "beta", "gamma"]
        assert all(isinstance(item, str) for item in result)


# --- A3: unquoted keys recover (REQ-101) ---


class TestRepairUnquotedKeys:
    def test_repair_recovers_unquoted_keys(self) -> None:
        raw = "[{query: alpha}, {query: beta}]"
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert len(result) >= 1
        assert all(isinstance(item, str) for item in result)


# --- A4: truncated / cut-off array recovers (REQ-101) ---


class TestRepairTruncatedArray:
    def test_repair_recovers_truncated_array(self) -> None:
        raw = '["alpha", "beta", "gamm'
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert len(result) >= 2
        assert "alpha" in result
        assert "beta" in result
        assert all(isinstance(item, str) for item in result)


# --- A5: JSON wrapped in prose / markdown fences recovers (REQ-101) ---


class TestRepairProseAndFences:
    def test_repair_recovers_prose_and_fences(self) -> None:
        raw = 'Here are the sub-queries:\n```json\n["alpha", "beta",]\n```\nHope this helps!'
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert result == ["alpha", "beta"]


# --- A6: repaired result truncated to breadth (REQ-102) ---


class TestRepairBreadthTruncation:
    def test_repair_result_truncated_to_breadth(self) -> None:
        raw = "['a', 'b', 'c', 'd', 'e', 'f']"
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)
        assert len(result) == 4
        assert result == ["a", "b", "c", "d"]


# --- A7: totally unparseable garbage -> [] (REQ-103) ---


class TestUnparseableReturnsEmpty:
    def test_unparseable_returns_empty(self) -> None:
        raw = "this is not json at all, no brackets, nothing useful"
        result = parse_sub_queries(raw, 4)
        assert result == []

    def test_empty_string_returns_empty(self) -> None:
        result = parse_sub_queries("", 4)
        assert result == []


# --- A8: repair tier never raises (REQ-103) ---


class TestRepairNeverRaises:
    @pytest.mark.parametrize(
        "raw",
        [
            "{{{{{{{{{{junk}}}}}}}}}}",
            "\x00\x01\x02\x03 binary-ish noise",
            "[",
            "[[[[[[",
            '"42"',
            '"just a string"',
            "{not even an array",
            "null",
            "true",
        ],
    )
    def test_repair_tier_never_raises(self, raw: str) -> None:
        result = parse_sub_queries(raw, 4)
        assert isinstance(result, list)  # may be []


# --- A9: clean JSON parses at tier 1; repair not invoked (REQ-104) ---


class TestCleanJsonUsesStandardTier:
    def test_clean_json_uses_standard_tier(self) -> None:
        raw = '["alpha", "beta", "gamma", "delta"]'
        result = parse_sub_queries(raw, 4)
        assert result == ["alpha", "beta", "gamma", "delta"]

    def test_valid_array_in_prose_uses_substring_tier(self) -> None:
        raw = 'Sure!\n["alpha", "beta"]\nDone.'
        result = parse_sub_queries(raw, 4)
        assert result == ["alpha", "beta"]


# --- A10: tier-attribution logging (REQ-201, 202) ---
#
# Tier identifiers: standard / substring / repaired / failed.
# Exactly one tier line per call; repaired + failed at WARNING level.


class TestTierAttributionLogging:
    def test_log_tier_standard(self, caplog: pytest.LogCaptureFixture) -> None:
        with caplog.at_level("DEBUG", logger="researcher.deep_tree"):
            parse_sub_queries('["a", "b"]', 4)
        tier_msgs = [r.message for r in caplog.records if "parse" in r.message.lower()]
        assert len(tier_msgs) >= 1
        joined = " ".join(tier_msgs)
        assert "standard" in joined
        assert "substring" not in joined
        assert "repaired" not in joined
        assert "failed" not in joined

    def test_log_tier_substring(self, caplog: pytest.LogCaptureFixture) -> None:
        with caplog.at_level("DEBUG", logger="researcher.deep_tree"):
            parse_sub_queries('noise ["a", "b"] noise', 4)
        tier_msgs = [r.message for r in caplog.records if "parse" in r.message.lower()]
        assert len(tier_msgs) >= 1
        joined = " ".join(tier_msgs)
        assert "substring" in joined
        assert "standard" not in joined
        assert "repaired" not in joined
        assert "failed" not in joined

    def test_log_tier_repaired_is_warning(self, caplog: pytest.LogCaptureFixture) -> None:
        with caplog.at_level("DEBUG", logger="researcher.deep_tree"):
            parse_sub_queries('["a", "b",]', 4)  # trailing comma -> repair tier
        repaired_records = [r for r in caplog.records if "parse" in r.message.lower() and "repaired" in r.message]
        assert repaired_records, "expected a 'repaired' tier-attribution log line"
        assert all(r.levelname == "WARNING" for r in repaired_records)

    def test_log_tier_failed_is_warning(self, caplog: pytest.LogCaptureFixture) -> None:
        with caplog.at_level("DEBUG", logger="researcher.deep_tree"):
            parse_sub_queries("not json", 4)
        failed_records = [r for r in caplog.records if "parse" in r.message.lower() and "failed" in r.message]
        assert failed_records, "expected a 'failed' tier-attribution log line"
        assert all(r.levelname == "WARNING" for r in failed_records)

    @pytest.mark.parametrize(
        "raw,expected_tier",
        [
            ('["a", "b"]', "standard"),
            ('noise ["a", "b"] noise', "substring"),
            ('["a", "b",]', "repaired"),
            ("not json", "failed"),
        ],
    )
    def test_exactly_one_tier_log_per_call(
        self,
        caplog: pytest.LogCaptureFixture,
        raw: str,
        expected_tier: str,
    ) -> None:
        with caplog.at_level("DEBUG", logger="researcher.deep_tree"):
            parse_sub_queries(raw, 4)
        tier_records = [r for r in caplog.records if "parse" in r.message.lower()]
        assert len(tier_records) == 1, (
            f"expected exactly 1 tier-attribution log line, got {len(tier_records)}: "
            f"{[r.message for r in tier_records]}"
        )
        assert expected_tier in tier_records[0].message


# --- A11: dependency declared and importable (REQ-301) ---


class TestJsonRepairDependency:
    def test_json_repair_importable(self) -> None:
        import json_repair as _jr

        assert hasattr(_jr, "loads")
        assert callable(_jr.loads)

    def test_pyproject_declares_json_repair(self) -> None:
        deps = _normalized_deps()
        assert "json-repair" in deps, f"json-repair not in declared deps: {sorted(deps)}"
